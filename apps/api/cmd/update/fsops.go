package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// The helpers below reproduce what the shell updater's coreutils calls did, down
// to the details that decide a file's final mode and owner. They are deliberately
// not "better" than the tools they replace: the previous updater is what live
// installs were upgraded with, so a quiet difference here is a behaviour change
// on the most dangerous path in the product.

func isRegularFile(path string) bool { // [[ -f path ]]
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular()
}

func isDir(path string) bool { // [[ -d path ]]
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func exists(path string) bool { // [[ -e path ]]
	_, err := os.Stat(path)
	return err == nil
}

// isExecutable is [[ -x path ]]: access(2) with X_OK, which for root is "any
// execute bit set", and which is also true of a directory.
func isExecutable(path string) bool {
	return syscall.Access(path, 1) == nil
}

// strerror is the C library's text for an errno, which is what coreutils prints:
// Go's is the same words in lower case.
func strerror(err error) string {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		s := errno.Error()
		return strings.ToUpper(s[:1]) + s[1:]
	}
	return err.Error()
}

// mkdirAll is `mkdir -p`: 0777 filtered by the umask. On failure it words the
// error as mkdir does, which names the first component in the way: an ancestor
// that is a file is "Not a directory", the path itself being a file is "File
// exists".
func mkdirAll(p string) error {
	err := os.MkdirAll(p, 0o777)
	if err == nil {
		return nil
	}
	cum := ""
	if strings.HasPrefix(p, "/") {
		cum = "/"
	}
	clean := path.Clean(p)
	for _, part := range strings.Split(strings.Trim(clean, "/"), "/") {
		cum = path.Join(cum, part)
		if fi, serr := os.Stat(cum); serr == nil && !fi.IsDir() {
			reason := "Not a directory"
			if cum == clean {
				reason = "File exists"
			}
			return fmt.Errorf("mkdir: cannot create directory '%s': %s", cum, reason)
		}
	}
	return fmt.Errorf("mkdir: cannot create directory '%s': %s", p, strerror(err))
}

// copyFile is `cp src dst` for a regular file, no -p. An existing dst is
// rewritten in place, so it keeps its own mode and owner; a new dst takes the
// source's mode filtered by the umask. That asymmetry is real cp behaviour and
// it is why a swapped-in infra file keeps whatever mode the operator gave the
// old one. A dst that is a directory gets the file put inside it, and a src that
// is a directory is refused without touching dst — both also what cp does.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cp: cannot stat '%s': %s", src, strerror(err))
	}
	defer func() { _ = in.Close() }()
	fi, err := in.Stat()
	if err != nil {
		return fmt.Errorf("cp: cannot stat '%s': %s", src, strerror(err))
	}
	if fi.IsDir() {
		return fmt.Errorf("cp: -r not specified; omitting directory '%s'", src)
	}
	if d, derr := os.Stat(dst); derr == nil && d.IsDir() {
		dst = dst + "/" + path.Base(src)
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fi.Mode().Perm())
	if err != nil {
		return fmt.Errorf("cp: cannot create regular file '%s': %s", dst, strerror(err))
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("cp: error writing '%s': %s", dst, strerror(err))
	}
	return out.Close()
}

// addExec is `chmod +x`. With no who-letter chmod adds the execute bits that the
// umask does not mask, so under 022 a 0644 file becomes 0755 but under 077 only
// 0744.
func addExec(p string, umask os.FileMode) error {
	fi, err := os.Stat(p)
	if err != nil {
		return fmt.Errorf("chmod: cannot access '%s': %s", p, strerror(err))
	}
	return chmod(p, fi.Mode().Perm()|(0o111&^umask))
}

// chmod is `chmod <octal> path`.
func chmod(p string, mode os.FileMode) error {
	if err := os.Chmod(p, mode); err != nil {
		return fmt.Errorf("chmod: changing permissions of '%s': %s", p, strerror(err))
	}
	return nil
}

// touch is `touch path`: create it empty (0666 filtered by the umask) if absent,
// otherwise just move its timestamps.
func touch(p string) error {
	now := time.Now()
	err := os.Chtimes(p, now, now)
	if err == nil {
		return nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		var f *os.File
		if f, err = os.OpenFile(p, os.O_WRONLY|os.O_CREATE, 0o666); err == nil {
			return f.Close()
		}
	}
	return fmt.Errorf("touch: cannot touch '%s': %s", p, strerror(err))
}

// chownIDs is `chown "uid:gid" path` with the ids exactly as `id -u` / `id -g`
// printed them. Follows symlinks, like chown without -h.
func chownIDs(p, uid, gid string) error {
	u, uerr := strconv.Atoi(uid)
	g, gerr := strconv.Atoi(gid)
	if uerr != nil || gerr != nil {
		return fmt.Errorf("chown: invalid user: '%s:%s'", uid, gid)
	}
	if err := os.Chown(p, u, g); err != nil {
		return fmt.Errorf("chown: changing ownership of '%s': %s", p, strerror(err))
	}
	return nil
}

// sameContent is `cmp -s a b`. An unreadable file counts as different, which is
// what `! cmp -s` made of cmp's "trouble" exit status.
func sameContent(a, b string) bool {
	x, err := os.ReadFile(a)
	if err != nil {
		return false
	}
	y, err := os.ReadFile(b)
	if err != nil {
		return false
	}
	return bytes.Equal(x, y)
}

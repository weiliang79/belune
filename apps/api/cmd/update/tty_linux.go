//go:build linux

package main

import (
	"syscall"
	"unsafe"
)

// stdinIsTerminal is `[[ -t 0 ]]`: isatty(0), i.e. whether TCGETS succeeds on
// fd 0.
//
// ⚠️ It must NOT be "is stdin a character device". /dev/null is one, and it is
// exactly what the dashboard's detached helper container gets as stdin — so a
// character-device test would call the helper interactive, read EOF from
// /dev/null as an empty reply, and abort with "Aborted." instead of the
// "no terminal" explanation the dashboard needs to show. Only a real terminal
// answers TCGETS.
func stdinIsTerminal() bool {
	var t syscall.Termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, 0, syscall.TCGETS, uintptr(unsafe.Pointer(&t)))
	return errno == 0
}

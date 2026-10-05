// update is /usr/local/bin/belune-update inside the Belune image, and it is the
// ONLY thing that moves an install between versions. It is not run directly:
// the dashboard (SpawnUpdateHelper) and scripts/update.sh both pull the TARGET
// image and run this binary out of it, so the updater that runs is always the one
// shipped with the version being installed — a fix to it reaches the very update
// that installs it, instead of the one after.
//
// Usage: belune-update <target-version>
//
// ⚠️ That contract — the path and the single argument — is frozen. Every
// launcher is old code on every future update and can never be repaired, so a Go
// binary and the shell script it replaced are interchangeable behind the
// entrypoint and nothing else about this interface may change.
//
// An update is a deliberate version move, never a drift: it resolves a target
// version, takes a backup before touching anything, rewrites the pinned image in
// .env, reconciles the version-pinned infra files (compose, Caddy, BuildKit,
// systemd, scripts) so a release that changes a service definition actually takes
// effect, and tells you exactly how to roll back if the new version does not come
// up. Migrations run automatically at boot and are not reversible, which is why
// the backup happens first.
//
// The infra files are a matched set with the image: they are fetched from the
// target release's git ref — never main/latest — exactly as install.sh does, so
// updating to a version yields the same files every time and rollback can restore
// the set that ran with the old image. Because a changed compose can touch any
// service (not just belune), the restart is a full `docker compose up -d`.
//
// This is a port of the shell updater, and "provably identical" was the bar.
// docker, curl, bash (scripts/backup.sh), systemctl and the final `cp -a` of a
// revert are still run as subprocesses — compose has no API, and keeping the
// process boundary is what lets internal/scripttest drive this binary against
// fake docker/curl on PATH. Do not move any of it onto the Docker SDK.
package main

import "os"

func main() {
	os.Exit(exitCode(newUpdater().Run(os.Args[1:])))
}

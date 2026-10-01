//go:build !linux

package main

// stdinIsTerminal exists only so the module still builds off Linux. The updater
// runs solely inside the Linux image; there is nothing to ask on elsewhere.
func stdinIsTerminal() bool { return false }

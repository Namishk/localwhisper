//go:build !windows

package main

// EnableClickThrough is a no-op off Windows; Linux uses the GTK overlay.
func EnableClickThrough(string) {}

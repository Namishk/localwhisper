//go:build !windows

package overlay

// EnableClickThrough is a no-op off Windows; Linux uses the GTK overlay.
func EnableClickThrough(string) {}

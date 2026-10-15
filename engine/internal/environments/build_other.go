//go:build !windows

package environments

// windowsBuild is 0 off Windows: unknown, and never refused for it.
var windowsBuild = func() uint32 { return 0 }

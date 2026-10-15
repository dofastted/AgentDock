//go:build windows

package environments

import "golang.org/x/sys/windows"

// windowsBuild is Windows' build number.
var windowsBuild = func() uint32 { return windows.RtlGetVersion().BuildNumber }

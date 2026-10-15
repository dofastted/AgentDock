package settings

import (
	"slices"
	"testing"
)

// The distros turned off are this computer's own: a sync or a restored
// backup from another machine neither brings its list nor clears this one.
func TestWSLDistrosOffOwn(t *testing.T) {
	from := Settings{Theme: "dark", WSLDistrosOff: []string{"Arch"}}
	from.KeepOwn(Settings{WSLDistrosOff: []string{"Debian"}})
	if !slices.Equal(from.WSLDistrosOff, []string{"Debian"}) || from.Theme != "dark" {
		t.Fatalf("KeepOwn: %+v", from)
	}
	from = Settings{WSLDistrosOff: []string{"Debian"}}
	from.KeepOwn(Settings{})
	if len(from.WSLDistrosOff) != 0 {
		t.Fatal("another computer's distro turned off here")
	}
}

// A distro is matched as wsl.exe names it, case aside; turning one on or
// off leaves the others, and says whether anything changed.
func TestSetWSLDistroOff(t *testing.T) {
	var s Settings
	if !s.SetWSLDistroOff("Debian", true) || !s.WSLDistroOff("debian") || s.WSLDistroOff("Ubuntu") {
		t.Fatalf("off: %+v", s.WSLDistrosOff)
	}
	if s.SetWSLDistroOff("DEBIAN", true) || len(s.WSLDistrosOff) != 1 {
		t.Fatalf("turned off twice: %+v", s.WSLDistrosOff)
	}
	s.SetWSLDistroOff("Ubuntu", true)
	if !s.SetWSLDistroOff("debian", false) || !slices.Equal(s.WSLDistrosOff, []string{"Ubuntu"}) {
		t.Fatalf("Debian back on: %+v", s.WSLDistrosOff)
	}
	if s.SetWSLDistroOff("Debian", false) {
		t.Fatal("a distro already on said to change")
	}
	s.SetWSLDistroOff("Ubuntu", false)
	if s.WSLDistrosOff != nil {
		t.Fatalf("an empty list is kept: %#v", s.WSLDistrosOff)
	}
}

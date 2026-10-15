package settings

import "strings"

// WSLDistroOff is whether distro is one of WSLDistrosOff.
func (s Settings) WSLDistroOff(distro string) bool {
	for _, d := range s.WSLDistrosOff {
		if strings.EqualFold(d, distro) {
			return true
		}
	}
	return false
}

// SetWSLDistroOff turns distro off or back on in WSLDistrosOff, keeping the
// others as they are; it says whether the list changed.
func (s *Settings) SetWSLDistroOff(distro string, off bool) bool {
	if s.WSLDistroOff(distro) == off {
		return false
	}
	if off {
		s.WSLDistrosOff = append(s.WSLDistrosOff, distro)
		return true
	}
	kept := s.WSLDistrosOff[:0:0]
	for _, d := range s.WSLDistrosOff {
		if !strings.EqualFold(d, distro) {
			kept = append(kept, d)
		}
	}
	if len(kept) == 0 {
		kept = nil
	}
	s.WSLDistrosOff = kept
	return true
}

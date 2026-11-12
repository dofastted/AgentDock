package main

import (
	"os"
	"strings"
	"testing"
)

// moejs compiles Intl's CLDR, Unicode, ICU and V8 material into every build
// and asks a program that includes it to carry sections 1 to 4 of its
// notices. The binary carries THIRD_PARTY_NOTICES; this holds it to the
// moejs version go.mod builds with, so an upgrade copies the new notices.
func TestNoticesFollowMoejs(t *testing.T) {
	b, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	var v string
	for _, l := range strings.Split(string(b), "\n") {
		if f := strings.Fields(l); len(f) >= 2 && f[0] == "github.com/MoeForge/moejs" {
			v = f[1]
		}
	}
	if v == "" {
		t.Fatal("go.mod requires no github.com/MoeForge/moejs")
	}
	if !strings.Contains(thirdParty, "THIRD_PARTY_NOTICES at "+v+",") {
		t.Errorf("THIRD_PARTY_NOTICES is not moejs %s's: copy sections 1 to 4 of its notices", v)
	}
	for _, s := range []string{"1. Unicode CLDR", "2. Unicode Character Database", "3. ICU", "4. V8"} {
		if !strings.Contains(thirdParty, "\n"+s) {
			t.Errorf("THIRD_PARTY_NOTICES has no section %q", s)
		}
	}
}

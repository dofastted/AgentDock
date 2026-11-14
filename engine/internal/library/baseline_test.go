package library

import (
	"slices"
	"testing"
)

func serverAgentsOf(t *testing.T, name string) []string {
	t.Helper()
	l, err := load()
	if err != nil {
		t.Fatal(err)
	}
	s := l.server(name)
	if s == nil {
		t.Fatalf("no server %s", name)
	}
	return s.Agents
}

// Followers get what the baseline has ticked, on setting it and on every
// change after: ticking or unticking the baseline ticks or unticks them.
func TestFollowersMirrorTheBaseline(t *testing.T) {
	sandbox(t)
	ok(t)(SaveServer("", Server{Name: "a", Transport: "stdio", Command: "a", Agents: []string{"omp@wsl:Debian"}}))
	ok(t)(SaveServer("", Server{Name: "b", Transport: "stdio", Command: "b", Agents: []string{"codex@wsl:Debian"}}))
	ok(t)(SetBaseline("omp@wsl:Debian", []string{"codex@wsl:Debian", "omp@wsl:Debian", "claude"}))
	if got := serverAgentsOf(t, "a"); !slices.Equal(got, []string{"claude", "codex@wsl:Debian", "omp@wsl:Debian"}) {
		t.Errorf("a: %v", got)
	}
	if got := serverAgentsOf(t, "b"); len(got) != 0 {
		t.Errorf("b, which the baseline hasn't, stays with a follower: %v", got)
	}
	b, _ := BaselineOf()
	if b.Agent != "omp@wsl:Debian" || !slices.Equal(b.Followers, []string{"claude", "codex@wsl:Debian"}) {
		t.Errorf("baseline: %+v", b)
	}
	ok(t)(ServerAgents("b", []string{"omp@wsl:Debian"}))
	if got := serverAgentsOf(t, "b"); !slices.Equal(got, []string{"claude", "codex@wsl:Debian", "omp@wsl:Debian"}) {
		t.Errorf("b after the baseline got it: %v", got)
	}
	ok(t)(ServerAgents("a", []string{}))
	if got := serverAgentsOf(t, "a"); len(got) != 0 {
		t.Errorf("a after the baseline lost it: %v", got)
	}
	// off: ticks stay as they are, and follow nothing
	ok(t)(SetBaseline("", nil))
	ok(t)(ServerAgents("b", []string{"omp@wsl:Debian"}))
	if got := serverAgentsOf(t, "b"); !slices.Equal(got, []string{"omp@wsl:Debian"}) {
		t.Errorf("b with no baseline: %v", got)
	}
}

func TestBaselineRefusesABadID(t *testing.T) {
	sandbox(t)
	if _, err := SetBaseline("omp@wsl:", nil); err == nil {
		t.Error("a bad baseline id was taken")
	}
	if _, err := SetBaseline("omp", []string{"not an id"}); err == nil {
		t.Error("a bad follower id was taken")
	}
}

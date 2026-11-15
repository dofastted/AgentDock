package library

// AgentDock: one agent is the user's main one (omp in WSL, say), and the
// others they pick follow it. Every change to the library gives each
// follower exactly what the baseline has ticked — instructions, servers,
// skills, hooks, subagents — so setting up the baseline sets up them all.
// A follower's own skill folder standing in the way of the library's by
// that name is kept aside with the backups and the library's put in its
// place, as UseLibrarySkill does when the user asks: a follower is the
// library's to manage. What a follower can't take (a Linux command for an
// agent on Windows with no windows variant) is said in the result, as for
// any tick.

import (
	"fmt"
	"slices"
	"strings"
)

// Baseline is the agent the followers mirror.
type Baseline struct {
	Agent     string   `json:"agent"`
	Followers []string `json:"followers,omitempty"`
}

// BaselineOf is the baseline set now; its Agent is "" when there is none.
func BaselineOf() (Baseline, error) {
	mu.Lock()
	defer mu.Unlock()
	l, err := load()
	if err != nil {
		return Baseline{}, err
	}
	if l.Baseline == nil {
		return Baseline{Followers: []string{}}, nil
	}
	b := *l.Baseline
	if b.Followers == nil {
		b.Followers = []string{}
	}
	return b, nil
}

// SetBaseline makes agent the baseline and followers its followers, and
// gives them its setup now; an empty agent turns the baseline off, leaving
// every agent's ticks as they are.
func SetBaseline(agent string, followers []string) (*Result, error) {
	if agent == "" {
		return change(func(l *Library) error { l.Baseline = nil; return nil })
	}
	if err := checkAgent(agent); err != nil {
		return nil, err
	}
	var fs []string
	for _, f := range followers {
		if f == agent || slices.Contains(fs, f) {
			continue
		}
		if err := checkAgent(f); err != nil {
			return nil, err
		}
		fs = append(fs, f)
	}
	slices.Sort(fs)
	return change(func(l *Library) error {
		l.Baseline = &Baseline{Agent: agent, Followers: fs}
		return nil
	})
}

// followBaseline gives the followers the baseline's ticks (changeLocked).
func (l *Library) followBaseline() {
	if b := l.Baseline; b != nil && b.Agent != "" {
		l.mirror(b.Agent, b.Followers)
	}
}

// liftToBaseline takes a tick the user changed on a follower, the
// baseline's own left as it was, onto the baseline, so followBaseline
// spreads it instead of putting the old one back: turning a skill off for
// claude (a follower) turned it straight back on. was is the skill's
// agents before the change.
func (l *Library) liftToBaseline(s *Skill, was []string) {
	b := l.Baseline
	if b == nil || b.Agent == "" || slices.Contains(was, b.Agent) != slices.Contains(s.Agents, b.Agent) {
		return
	}
	for _, f := range b.Followers {
		if on := slices.Contains(s.Agents, f); on != slices.Contains(was, f) {
			s.Agents = set(s.Agents, b.Agent, on)
			return
		}
	}
}

// settleFollowers puts the library's skill in place of a follower's own
// one that stood in its way in res, and says whether it did any (the
// agents are then written again). Claude Desktop's own skills are its
// manifest's, never put aside.
func (l *Library) settleFollowers(res *Result) bool {
	b := l.Baseline
	if b == nil || b.Agent == "" || res == nil {
		return false
	}
	settled := false
	for _, p := range res.Problems {
		name, ok := strings.CutPrefix(p.What, "skill:")
		if !ok || !p.Own || !slices.Contains(b.Followers, p.Agent) {
			continue
		}
		if err := useLibrarySkill(l, name, p.Agent); err != nil {
			res.Problems = append(res.Problems, Problem{Agent: p.Agent, What: p.What, Error: fmt.Sprintf("couldn't put its own aside for the library's: %v", err)})
			continue
		}
		settled = true
	}
	return settled
}

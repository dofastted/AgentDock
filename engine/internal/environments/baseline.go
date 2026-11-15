package environments

import (
	"github.com/yetone/magpie/internal/library"
)

// BaselinePreview is what making agent the baseline of followers would
// change in what the library gives each follower: nothing is written.
// Only the library's ticks are compared; each agent's own model, sign-in
// and connection stay its own (that is Copy an agent's setup).
func BaselinePreview(agent string, followers []string) (Plan, error) {
	src, err := library.AgentBindings(agent)
	if err != nil {
		return Plan{}, err
	}
	p := Plan{Source: agent, Targets: []TargetPlan{}}
	for _, f := range followers {
		if f == agent {
			continue
		}
		has, err := library.AgentBindings(f)
		if err != nil {
			return Plan{}, err
		}
		ch := libraryChanges(src, has)
		if ch == nil {
			ch = []Change{}
		}
		p.Targets = append(p.Targets, TargetPlan{Agent: f, Changes: ch})
	}
	return p, nil
}

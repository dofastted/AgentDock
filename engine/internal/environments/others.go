package environments

import (
	"context"
	"os"
	"time"

	"github.com/yetone/magpie/internal/proc"
)

// OtherMagpie is a magpie on this computer other than AgentDock's engine:
// it would rewrite the same agents' files, and may hold the gateway's port.
type OtherMagpie struct {
	PID  int    `json:"pid"`
	Args string `json:"args"`
}

// OtherMagpies are the magpies running on this computer besides this
// process; ok is false when the system couldn't be asked.
func OtherMagpies() (others []OtherMagpie, ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ps, err := proc.List(ctx, "magpie")
	if err != nil {
		return nil, false
	}
	self := os.Getpid()
	others = []OtherMagpie{}
	for _, p := range ps {
		exe, _ := proc.Executable(p.PID)
		if p.PID == self || exe != "" && !proc.IsMagpie(exe) {
			continue
		}
		others = append(others, OtherMagpie{PID: p.PID, Args: p.Args})
	}
	return others, true
}

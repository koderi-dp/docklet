//go:build darwin

package ports

import (
	"context"
	"fmt"
)

type darwinHost struct{ run runFunc }

// NewLocalHost reads this Mac's listeners with lsof (names and owners only for programs
// the current user may inspect).
func NewLocalHost() HostSource { return darwinHost{run: execRun} }

func (darwinHost) Where() string { return "This machine" }

func (h darwinHost) Listeners(ctx context.Context) ([]Listener, error) {
	var out []Listener
	for _, args := range [][]string{{"-nP", "-iTCP", "-sTCP:LISTEN"}, {"-nP", "-iUDP"}} {
		b, err := h.run(ctx, "lsof", args...)
		if err != nil && !(len(b) == 0 && exitCode(err) == 1) { // lsof exits 1 when nothing matches
			return nil, fmt.Errorf("lsof: %w", err)
		}
		out = append(out, ParseLsof(string(b), "This machine")...)
	}
	return out, nil
}

package ports

import "context"

// Suggestion is the next port above the requested one that nothing holds. Verified is
// false when the check could only rule out containers (the Engine is on another machine).
type Suggestion struct {
	Port     int    `json:"port"` // 0 when nothing suitable was found
	Verified bool   `json:"verified"`
	Detail   string `json:"detail"`
}

const suggestSpan = 200

// NextFree looks upward from req.HostPort+1 using one snapshot of the sources.
func (c *Checker) NextFree(ctx context.Context, req Request, ignore []string) Suggestion {
	s, cancel := c.snap(ctx)
	defer cancel()
	skip := toSet(ignore)
	var firstUnknown *PortCheck
	for p := req.HostPort + 1; p <= 65535 && p <= req.HostPort+suggestSpan; p++ {
		r := req
		r.HostPort = p
		res := s.evaluate(r, skip)
		switch res.Status {
		case StatusFree:
			return Suggestion{Port: p, Verified: true}
		case StatusUnknown:
			if firstUnknown == nil {
				firstUnknown = &res
			}
		}
	}
	if firstUnknown != nil {
		return Suggestion{Port: firstUnknown.HostPort, Detail: firstUnknown.Detail}
	}
	return Suggestion{Detail: "No free port was found above the requested one."}
}

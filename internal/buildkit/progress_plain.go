package buildkit

import (
	"regexp"
	"strconv"
	"strings"
)

// Plain progress (`--progress=plain`): every line is "#<id> <text>". The first
// text of an id is the step name; "DONE 1.2s", "CACHED", "ERROR: ..." and
// "CANCELED" end it; anything else is log output of that step. Lines without a
// "#<id>" prefix are the CLI's own final output.
var (
	plainLineRe = regexp.MustCompile(`^#(\d+) (.*)$`)
	plainDoneRe = regexp.MustCompile(`^DONE (\d+(?:\.\d+)?)s$`)
	plainLogRe  = regexp.MustCompile(`^\d+\.\d+ (.*)$`)
)

func (t *Tracker) feedPlain(line string) {
	m := plainLineRe.FindStringSubmatch(line)
	if m == nil || m[1] == "0" { // #0 is the builder banner
		return
	}
	s := t.get("#" + m[1])
	text := t.clean(m[2])
	switch {
	case !s.started:
		s.name = text
		t.startStep(s)
	case plainDoneRe.MatchString(text):
		f, _ := strconv.ParseFloat(plainDoneRe.FindStringSubmatch(text)[1], 64)
		s.completed, s.durMs = true, int64(f*1000)
	case text == "CACHED":
		s.completed, s.cached = true, true
	case text == "CANCELED" || text == "CANCELLED":
		s.canceled = true
	case strings.HasPrefix(text, "ERROR: "):
		s.err = strings.TrimPrefix(text, "ERROR: ")
		t.emit("    ERROR: " + s.err)
	default:
		if lm := plainLogRe.FindStringSubmatch(text); lm != nil {
			text = lm[1]
		}
		t.addLog(s, text)
	}
}

package compose

import (
	"testing"
)

func TestNormalizeProjectName_MatchesComposeRules(t *testing.T) {
	for in, want := range map[string]string{"My App": "myapp", "My_App-2": "my_app-2", "_-x": "x", "...": ""} {
		if got := NormalizeProjectName(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

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

func TestValidateProfile_AcceptsOnlyTheExactWildcard(t *testing.T) {
	for _, ok := range []string{"*", "dev", "web-1"} {
		if err := ValidateProfile(ok); err != nil {
			t.Errorf("%q should be valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"a*", "**", "", "-x", "*a"} {
		if err := ValidateProfile(bad); err == nil {
			t.Errorf("%q should be invalid", bad)
		}
	}
}

package notify

import (
	"context"
	"strings"
	"testing"
)

func TestLastHealthOutput(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name, inspect, want string
	}{
		{"newest run wins", `{"State":{"Health":{"Status":"unhealthy","Log":[{"Output":"first"},{"Output":"second"}]}}}`, "second"},
		{"only the last non-empty line, redacted", `{"State":{"Health":{"Log":[{"Output":"warming up\ndial failed password=hunter2\n\n"}]}}}`, "dial failed password=***"},
		{"no healthcheck", `{"State":{"Running":true}}`, ""},
		{"empty log", `{"State":{"Health":{"Status":"starting","Log":[]}}}`, ""},
		{"newest run printed nothing", `{"State":{"Health":{"Log":[{"Output":"older"},{"Output":""}]}}}`, ""},
		{"no state", `{"Id":"x"}`, ""},
		{"long output is truncated", `{"State":{"Health":{"Log":[{"Output":"` + strings.Repeat("y", 400) + `"}]}}}`, strings.Repeat("y", detailMaxLen) + "..."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := lastHealthOutput(ctx, inspectProvider(t, tc.inspect), "c1", "abc"); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestLastHealthOutput_UnreachableEngineYieldsEmpty(t *testing.T) {
	if got := lastHealthOutput(context.Background(), failingProvider{}, "c1", "abc"); got != "" {
		t.Fatalf("got %q", got)
	}
	// a client whose server answered 404 for the container
	if got := lastHealthOutput(context.Background(), inspectProviderStatus(t, 404), "c1", "gone"); got != "" {
		t.Fatalf("got %q", got)
	}
}

package runspec

import "testing"

func TestQuote(t *testing.T) {
	cases := []struct {
		in, bash, pwsh string
	}{
		{"plain-1.2/x:y=z", "plain-1.2/x:y=z", "plain-1.2/x:y=z"},
		{"", "''", "''"},
		{"a b", "'a b'", "'a b'"},
		{"it's", `'it'\''s'`, "'it''s'"},
		{"a,b", "a,b", "'a,b'"},         // comma is an array operator in PowerShell
		{"@x", "@x", "'@x'"},            // splatting in PowerShell
		{"$HOME", "'$HOME'", "'$HOME'"}, // never expanded
		{`say "hi"`, `'say "hi"'`, `'say "hi"'`},
		{"\u2018smart\u2019", "'\u2018smart\u2019'", "'\u2018\u2018smart\u2019\u2019'"},
		{`C:\data`, `'C:\data'`, `'C:\data'`},
		{"line1\nline2", "'line1\nline2'", "'line1\nline2'"},
	}
	for _, c := range cases {
		if got := Bash.Quote(c.in); got != c.bash {
			t.Errorf("bash Quote(%q) = %s, want %s", c.in, got, c.bash)
		}
		if got := PowerShell.Quote(c.in); got != c.pwsh {
			t.Errorf("pwsh Quote(%q) = %s, want %s", c.in, got, c.pwsh)
		}
	}
}

func TestSecretEnvArg(t *testing.T) {
	if got := Bash.secretEnvArg("DB_PASSWORD"); got != `DB_PASSWORD="$DB_PASSWORD"` {
		t.Errorf("bash: %s", got)
	}
	if got := PowerShell.secretEnvArg("DB_PASSWORD"); got != `"DB_PASSWORD=$env:DB_PASSWORD"` {
		t.Errorf("pwsh: %s", got)
	}
	if got := Bash.secretEnvArg("my.key"); got != `my.key="$my_key"` {
		t.Errorf("odd key: %s", got)
	}
}

func TestFormatBytesAndPorts(t *testing.T) {
	for in, want := range map[int64]string{512 << 20: "512m", 2 << 30: "2g", 1500: "1500", 64 << 10: "64k"} {
		if got := formatBytes(in); got != want {
			t.Errorf("formatBytes(%d) = %s, want %s", in, got, want)
		}
	}
	ports := map[string]Port{
		"80":                {ContainerPort: 80, Proto: "tcp"},
		"8080:80":           {HostPort: "8080", ContainerPort: 80, Proto: "tcp"},
		"127.0.0.1:8080:80": {HostIP: "127.0.0.1", HostPort: "8080", ContainerPort: 80, Proto: "tcp"},
		"53:53/udp":         {HostPort: "53", ContainerPort: 53, Proto: "udp"},
		"[::1]:8080:80":     {HostIP: "::1", HostPort: "8080", ContainerPort: 80, Proto: "tcp"},
		"127.0.0.1::80":     {HostIP: "127.0.0.1", ContainerPort: 80, Proto: "tcp"},
	}
	for want, p := range ports {
		if got := portSpec(p); got != want {
			t.Errorf("portSpec = %s, want %s", got, want)
		}
	}
}

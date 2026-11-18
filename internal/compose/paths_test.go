package compose

import (
	"strings"
	"testing"
)

func TestConvertWSLPath_AcceptsLinuxAndUNCPathsAndRejectsWindowsOnes(t *testing.T) {
	cases := []struct{ in, want, bad string }{
		{"/home/u/app/compose.yaml", "/home/u/app/compose.yaml", ""},
		{"/home/u/app/../app/./c.yml", "/home/u/app/c.yml", ""},
		{`\\wsl$\Ubuntu\home\u\app\compose.yaml`, "/home/u/app/compose.yaml", ""},
		{`\\wsl.localhost\ubuntu\home\u\c.yml`, "/home/u/c.yml", ""},
		{"//wsl$/Ubuntu/srv/c.yml", "/srv/c.yml", ""},
		{`\\wsl$\Ubuntu`, "/", ""},
		{`\\wsl$\Debian\home\c.yml`, "", "Debian"},
		{`\\wsl$\`, "", "does not name"},
		{`C:\Users\me\compose.yml`, "", "Windows path"},
		{`d:/x/compose.yml`, "", "Windows path"},
		{`\\server\share\c.yml`, "", "network share"},
		{"relative/c.yml", "", "not an absolute"},
		{"", "", "empty"},
		{"-f", "", "must not start"},
		{"/a\nb", "", "control"},
	}
	for _, c := range cases {
		got, err := ConvertWSLPath("Ubuntu", c.in)
		if c.bad != "" {
			if err == nil || !strings.Contains(err.Error(), c.bad) {
				t.Errorf("%q: want error containing %q, got %q %v", c.in, c.bad, got, err)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("%q: got %q %v want %q", c.in, got, err, c.want)
		}
	}
}

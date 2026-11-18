package wsldocker

import "testing"

func TestEnableSystemd_KeepsOtherContentAndIsIdempotent(t *testing.T) {
	cases := map[string]string{
		"":                                  "[boot]\nsystemd=true\n",
		"[user]\ndefault=alice\n":           "[user]\ndefault=alice\n\n[boot]\nsystemd=true\n",
		"[boot]\ncommand=service x start\n": "[boot]\ncommand=service x start\nsystemd=true\n",
		"[boot]\nsystemd=false\n[user]\ndefault=a\n":      "[boot]\nsystemd=true\n[user]\ndefault=a\n",
		"[boot]\ncommand=x\n\n[network]\nhostname=h\n":    "[boot]\ncommand=x\nsystemd=true\n\n[network]\nhostname=h\n",
		"[network]\nhostname=h\n[boot]\nsystemd = true\n": "[network]\nhostname=h\n[boot]\nsystemd=true\n",
		"[user]\r\ndefault=a\r\n":                         "[user]\ndefault=a\n\n[boot]\nsystemd=true\n",
	}
	for in, want := range cases {
		got := EnableSystemd(in)
		if got != want {
			t.Errorf("input %q:\n got %q\nwant %q", in, got, want)
		}
		if again := EnableSystemd(got); again != got {
			t.Errorf("input %q is not idempotent: %q", in, again)
		}
	}
}

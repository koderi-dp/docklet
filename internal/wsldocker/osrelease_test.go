package wsldocker

import "testing"

func TestParseOSRelease_ReadsQuotedValuesAndToleratesMissingFields(t *testing.T) {
	o := ParseOSRelease(ubuntuOSRelease)
	if o.ID != "ubuntu" || o.Codename() != "noble" || o.VersionID != "24.04" || o.PrettyName != "Ubuntu 24.04.1 LTS" {
		t.Fatalf("ubuntu: %+v", o)
	}
	if len(o.IDLike) != 1 || o.IDLike[0] != "debian" {
		t.Fatalf("id_like: %+v", o.IDLike)
	}
	o = ParseOSRelease("# comment\r\nID='debian'\r\n\r\nGARBAGE\r\n")
	if o.ID != "debian" || o.Codename() != "" || o.VersionID != "" {
		t.Fatalf("minimal: %+v", o)
	}
	if o := ParseOSRelease(""); o.ID != "" || o.Family() != "" {
		t.Fatalf("empty: %+v", o)
	}
}

func TestOSRelease_DocsURLFollowsFamily(t *testing.T) {
	cases := map[string]string{
		"ID=fedora": "https://docs.docker.com/engine/install/fedora/",
		"ID=rocky\nID_LIKE=\"rhel centos fedora\"": "https://docs.docker.com/engine/install/rhel/",
		"ID=linuxmint\nID_LIKE=\"ubuntu debian\"":  "https://docs.docker.com/engine/install/ubuntu/",
		"ID=alpine": "https://docs.docker.com/engine/install/",
	}
	for text, want := range cases {
		if got := ParseOSRelease(text).DocsURL(); got != want {
			t.Errorf("%q: got %s want %s", text, got, want)
		}
	}
}

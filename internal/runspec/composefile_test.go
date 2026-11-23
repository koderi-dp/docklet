package runspec

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

func services(t *testing.T, text string) map[string]any {
	t.Helper()
	svcs, _ := parseCompose(t, text)["services"].(map[string]any)
	return svcs
}

func svcOf(t *testing.T, text, name string) map[string]any {
	t.Helper()
	s, ok := services(t, text)[name].(map[string]any)
	if !ok {
		t.Fatalf("service %q missing:\n%s", name, text)
	}
	return s
}

func TestToComposeFile_StartsWithNameAndMergesServices(t *testing.T) {
	f := ToComposeFile("shop", []Spec{{Name: "web", Image: "nginx:1.27"}, {Name: "db", Image: "postgres:16"}}, Options{})
	if !strings.HasPrefix(f.Text, "name: shop\nservices:\n") {
		t.Fatalf("unexpected start:\n%s", f.Text)
	}
	if svcOf(t, f.Text, "web")["image"] != "nginx:1.27" || svcOf(t, f.Text, "db")["image"] != "postgres:16" {
		t.Fatalf("services:\n%s", f.Text)
	}
	if strings.Index(f.Text, "  web:") > strings.Index(f.Text, "  db:") {
		t.Fatalf("services must keep the given order:\n%s", f.Text)
	}
}

func TestToComposeFile_UniqueServiceNames(t *testing.T) {
	f := ToComposeFile("shop", []Spec{{Name: "app", Image: "a"}, {Name: "app", Image: "b"}, {Name: "app", Image: "c"}}, Options{})
	s := services(t, f.Text)
	if len(s) != 3 || s["app"] == nil || s["app-2"] == nil || s["app-3"] == nil {
		t.Fatalf("got %v", s)
	}
}

func foreignDB() Spec {
	return Spec{
		Name: "old-db-1", Image: "postgres:16", ComposeProject: "old", ComposeService: "db",
		Mounts:   []Mount{{Type: "volume", Source: "old_data", Target: "/var/lib/postgresql/data"}},
		Networks: []NetAttach{{Name: "old_default"}},
	}
}

func TestToComposeFile_KeepsExistingDataForForeignProject(t *testing.T) {
	f := ToComposeFile("shop", []Spec{foreignDB()}, Options{})
	doc := parseCompose(t, f.Text)
	vol, _ := doc["volumes"].(map[string]any)["old_data"].(map[string]any)
	if vol["name"] != "old_data" {
		t.Errorf("the existing volume must be reused by name:\n%s", f.Text)
	}
	net, _ := doc["networks"].(map[string]any)["old_default"].(map[string]any)
	if net["external"] != true {
		t.Errorf("the existing network must be external:\n%s", f.Text)
	}
	db := svcOf(t, f.Text, "db")
	if db["container_name"] != "old-db-1" {
		t.Errorf("container_name: %v", db["container_name"])
	}
	if !reflect.DeepEqual(db["volumes"], []any{"old_data:/var/lib/postgresql/data"}) {
		t.Errorf("volumes: %v", db["volumes"])
	}
}

func TestToComposeFile_SameProjectStaysLocal(t *testing.T) {
	f := ToComposeFile("old", []Spec{foreignDB()}, Options{})
	doc := parseCompose(t, f.Text)
	vol, ok := doc["volumes"].(map[string]any)["data"]
	if !ok {
		t.Fatalf("volume key must be the project-local one:\n%s", f.Text)
	}
	if m, _ := vol.(map[string]any); m["name"] != nil {
		t.Errorf("a project-local volume takes no name: %v", m)
	}
	if _, ok := svcOf(t, f.Text, "db")["container_name"]; ok {
		t.Errorf("no container_name inside its own project:\n%s", f.Text)
	}
	if doc["networks"] != nil {
		t.Errorf("the project's default network is implicit:\n%s", f.Text)
	}
}

func TestToComposeFile_ClashingVolumeKeys(t *testing.T) {
	a := Spec{Name: "shop-a-1", Image: "a", ComposeProject: "shop", ComposeService: "a", Mounts: []Mount{{Type: "volume", Source: "shop_data", Target: "/a"}}}
	b := Spec{Name: "b", Image: "b", Mounts: []Mount{{Type: "volume", Source: "data", Target: "/x"}}}
	f := ToComposeFile("shop", []Spec{a, b}, Options{})
	vols := parseCompose(t, f.Text)["volumes"].(map[string]any)
	if m, _ := vols["data"].(map[string]any); m["name"] != nil {
		t.Errorf("first volume keeps the local key without a name: %v", vols["data"])
	}
	if m, _ := vols["data-2"].(map[string]any); m["name"] != "data" {
		t.Errorf("the clashing volume gets a new key that still names the existing volume: %v", vols)
	}
	if got := svcOf(t, f.Text, "b")["volumes"]; !reflect.DeepEqual(got, []any{"data-2:/x"}) {
		t.Errorf("the service must reference the new key: %v", got)
	}
	// The same volume used twice is one key, not a clash.
	c := Spec{Name: "c", Image: "c", Mounts: []Mount{{Type: "volume", Source: "data", Target: "/y"}}}
	f = ToComposeFile("other", []Spec{b, c}, Options{})
	if vols := parseCompose(t, f.Text)["volumes"].(map[string]any); len(vols) != 1 {
		t.Errorf("one shared volume expected: %v", vols)
	}
}

func TestToComposeFile_ReferencesBetweenTickedAndDangling(t *testing.T) {
	web := Spec{Name: "web", Image: "nginx", Links: []string{"db:database", "ghost"}, VolumesFrom: []string{"db", "gone:ro"}}
	db := Spec{Name: "db", Image: "postgres"}
	sidecar := Spec{Name: "sidecar", Image: "busybox", NetworkMode: "container:web"}
	lost := Spec{Name: "lost", Image: "busybox", NetworkMode: "container:abc123"}
	f := ToComposeFile("shop", []Spec{web, db, sidecar, lost}, Options{})
	w := svcOf(t, f.Text, "web")
	if !reflect.DeepEqual(w["links"], []any{"db:database"}) || !reflect.DeepEqual(w["volumes_from"], []any{"db"}) {
		t.Errorf("web: links %v volumes_from %v", w["links"], w["volumes_from"])
	}
	if got := svcOf(t, f.Text, "sidecar")["network_mode"]; got != "service:web" {
		t.Errorf("sidecar network_mode: %v", got)
	}
	if _, ok := svcOf(t, f.Text, "lost")["network_mode"]; ok {
		t.Errorf("a dangling network mode must be left out:\n%s", f.Text)
	}
	for _, want := range []string{
		"web: link ghost (not in this file)",
		"web: volumes from gone (not in this file)",
		"lost: network mode container:abc123 (not in this file)",
	} {
		if !slices.Contains(f.NotIncluded, want) {
			t.Errorf("missing note %q in %v", want, f.NotIncluded)
		}
	}
}

func secretSpec(name, key, val string) Spec {
	return Spec{Name: name, Image: "x", Env: []EnvVar{{Key: key, Value: val, Secret: true}, {Key: "MODE", Value: "prod"}}}
}

func envOf(t *testing.T, text, svc string) map[string]any {
	t.Helper()
	return svcOf(t, text, svc)["environment"].(map[string]any)
}

func TestToComposeFile_SecretPlaceholders(t *testing.T) {
	one := ToComposeFile("shop", []Spec{secretSpec("db", "POSTGRES_PASSWORD", "pw")}, Options{})
	if got := envOf(t, one.Text, "db")["POSTGRES_PASSWORD"]; got != "${POSTGRES_PASSWORD}" {
		t.Errorf("single service: %v", got)
	}
	if !reflect.DeepEqual(one.SecretKeys, []string{"db.POSTGRES_PASSWORD"}) {
		t.Errorf("secret keys: %v", one.SecretKeys)
	}

	two := ToComposeFile("shop", []Spec{secretSpec("api", "PASSWORD", "a$1"), secretSpec("db-main", "PASSWORD", "b")}, Options{})
	if got := envOf(t, two.Text, "api")["PASSWORD"]; got != "${API_PASSWORD}" {
		t.Errorf("api: %v", got)
	}
	if got := envOf(t, two.Text, "db-main")["PASSWORD"]; got != "${DB_MAIN_PASSWORD}" {
		t.Errorf("db-main: %v", got)
	}
	if strings.Contains(two.Text, "a$") {
		t.Errorf("a secret value leaked:\n%s", two.Text)
	}

	open := ToComposeFile("shop", []Spec{secretSpec("api", "PASSWORD", "a$1"), secretSpec("db-main", "PASSWORD", "b")}, Options{IncludeSecrets: true})
	if got := envOf(t, open.Text, "api")["PASSWORD"]; got != "a$$1" {
		t.Errorf("included secret must be literal with dollars doubled: %v", got)
	}
	if len(open.SecretKeys) != 2 {
		t.Errorf("secret keys are reported either way: %v", open.SecretKeys)
	}
}

func TestToComposeFile_NotIncludedCommentAndPrefix(t *testing.T) {
	f := ToComposeFile("shop", []Spec{{Name: "web", Image: "nginx", AutoRemove: true}}, Options{})
	if !reflect.DeepEqual(f.NotIncluded, []string{"web: auto-remove (--rm)"}) {
		t.Fatalf("notes: %v", f.NotIncluded)
	}
	if !strings.HasSuffix(f.Text, "# not included: web: auto-remove (--rm)\n") {
		t.Fatalf("comment missing:\n%s", f.Text)
	}
}

func TestToComposeFile_NoSpecs(t *testing.T) {
	f := ToComposeFile("shop", nil, Options{})
	if f.Text != "name: shop\nservices: {}\n" {
		t.Fatalf("got %q", f.Text)
	}
	if f.NotIncluded == nil || f.SecretKeys == nil {
		t.Fatal("slices must not be nil")
	}
}

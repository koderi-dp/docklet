package runspec

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func parseCompose(t *testing.T, text string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		t.Fatalf("invalid YAML: %v\n%s", err, text)
	}
	return doc
}

func TestComposeRich(t *testing.T) {
	out := ToCompose(richSpec(), Options{})
	doc := parseCompose(t, out)
	svc := doc["services"].(map[string]any)["web"].(map[string]any)
	env := svc["environment"].(map[string]any)
	if env["DB_PASSWORD"] != "${DB_PASSWORD}" {
		t.Errorf("secret must be a variable reference, got %v", env["DB_PASSWORD"])
	}
	if env["PRICE"] != "$$5" {
		t.Errorf("literal dollars must be escaped, got %v", env["PRICE"])
	}
	ports := svc["ports"].([]any)
	if ports[0] != "127.0.0.1:8080:80" || ports[1] != "53:53/udp" {
		t.Errorf("ports %v (must stay strings, not sexagesimal numbers)", ports)
	}
	if svc["container_name"] != "web" || svc["restart"] != "on-failure:3" || svc["mem_limit"] != "512m" || svc["cpus"] != 1.5 {
		t.Errorf("service: %v", svc)
	}
	nets := svc["networks"].(map[string]any)
	if _, ok := nets["appnet"].(map[string]any)["aliases"]; !ok {
		t.Errorf("aliases lost: %v", nets)
	}
	top := doc["networks"].(map[string]any)["appnet"].(map[string]any)
	if top["external"] != true {
		t.Errorf("a network not created by a Compose project must be external: %v", top)
	}
	if v := doc["volumes"].(map[string]any)["pgdata"].(map[string]any); v["name"] != "pgdata" {
		t.Errorf("named volume: %v", v)
	}
	// Order is conventional: image first, then container_name.
	if !(strings.Index(out, "image:") < strings.Index(out, "container_name:") && strings.Index(out, "container_name:") < strings.Index(out, "environment:")) {
		t.Errorf("unexpected key order:\n%s", out)
	}
	if !strings.HasSuffix(out, "# not included: annotations\n") {
		t.Errorf("not-included note missing:\n%s", out)
	}
}

func TestComposeSecretsExplicit(t *testing.T) {
	out := ToCompose(richSpec(), Options{IncludeSecrets: true})
	env := parseCompose(t, out)["services"].(map[string]any)["web"].(map[string]any)["environment"].(map[string]any)
	if env["DB_PASSWORD"] != "s3cr3t-Value!$$x" {
		t.Errorf("value (dollar escaped) expected, got %v", env["DB_PASSWORD"])
	}
}

func TestComposeProjectNetworksAndVolumes(t *testing.T) {
	s := Spec{Name: "shop-api-1", Image: "api", ComposeProject: "shop", ComposeService: "api", NetworkMode: "shop_default",
		Networks: []NetAttach{{Name: "shop_default"}, {Name: "shop_back"}, {Name: "shared"}},
		Mounts:   []Mount{{Type: "volume", Source: "shop_data", Target: "/data"}, {Type: "volume", Target: "/anon"}}}
	doc := parseCompose(t, ToCompose(s, Options{}))
	svc := doc["services"].(map[string]any)["api"].(map[string]any)
	if _, ok := svc["container_name"]; ok {
		t.Errorf("generated compose container names must not be pinned")
	}
	nets := doc["networks"].(map[string]any)
	if _, ok := nets["default"]; ok {
		t.Errorf("implicit default network must not be declared: %v", nets)
	}
	if nets["back"].(map[string]any)["external"] != nil {
		t.Errorf("project network must not be external: %v", nets["back"])
	}
	if nets["shared"].(map[string]any)["external"] != true {
		t.Errorf("foreign network must be external: %v", nets["shared"])
	}
	if _, ok := svc["networks"].([]any); !ok {
		t.Errorf("two networks expected: %v", svc["networks"])
	}
	vols := svc["volumes"].([]any)
	if vols[0] != "data:/data" || vols[1] != "/anon" {
		t.Errorf("volumes %v", vols)
	}
}

func TestComposeEntrypointAndHealthAndHost(t *testing.T) {
	s := Spec{Name: "h", Image: "x", NetworkMode: "host", EntrypointSet: true, Entrypoint: []string{}, Health: &Health{Disabled: true}}
	svc := parseCompose(t, ToCompose(s, Options{}))["services"].(map[string]any)["h"].(map[string]any)
	if ep, ok := svc["entrypoint"].([]any); !ok || len(ep) != 0 {
		t.Errorf("cleared entrypoint must be an empty list: %v", svc["entrypoint"])
	}
	if svc["network_mode"] != "host" || svc["healthcheck"].(map[string]any)["disable"] != true {
		t.Errorf("service: %v", svc)
	}
}

func TestComposeOnlyImplicitNetworkIsOmitted(t *testing.T) {
	s := Spec{Name: "a-1", Image: "x", ComposeProject: "p", ComposeService: "a", NetworkMode: "p_default", Networks: []NetAttach{{Name: "p_default"}}}
	doc := parseCompose(t, ToCompose(s, Options{}))
	if _, ok := doc["services"].(map[string]any)["a"].(map[string]any)["networks"]; ok || doc["networks"] != nil {
		t.Errorf("implicit network written: %v", doc)
	}
}

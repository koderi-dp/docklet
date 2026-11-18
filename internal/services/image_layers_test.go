package services

import (
	"testing"

	"docklet/internal/model"
)

func TestParseCreatedBy(t *testing.T) {
	cases := []struct{ in, kind, instr, cmd string }{
		{"/bin/sh -c #(nop) ADD file:abc123 in / ", "ADD", "ADD file:abc123 in /", "file:abc123 in /"},
		{`/bin/sh -c #(nop)  CMD ["bash"]`, "CMD", `CMD ["bash"]`, `["bash"]`},
		{"/bin/sh -c #(nop)  ENV PG_MAJOR=16", "ENV", "ENV PG_MAJOR=16", "PG_MAJOR=16"},
		{"/bin/sh -c #(nop)  EXPOSE 5432", "OTHER", "EXPOSE 5432", "EXPOSE 5432"},
		{"/bin/sh -c apt-get update && apt-get install -y curl", "RUN", "RUN apt-get update && apt-get install -y curl", "apt-get update && apt-get install -y curl"},
		{"|2 VERSION=1.2 TARGET=x /bin/sh -c make build", "RUN", "RUN make build", "make build"},
		{"RUN /bin/sh -c set -eux; apt-get update # buildkit", "RUN", "RUN set -eux; apt-get update", "set -eux; apt-get update"},
		{"RUN |1 VERSION=1.2 /bin/sh -c make build # buildkit", "RUN", "RUN make build", "make build"},
		{"COPY . /app # buildkit", "COPY", "COPY . /app", ". /app"},
		{"WORKDIR /app", "OTHER", "WORKDIR /app", "WORKDIR /app"},
		{"ENV NODE_ENV=production", "ENV", "ENV NODE_ENV=production", "NODE_ENV=production"},
		{"", "OTHER", "", ""},
	}
	for _, c := range cases {
		k, i, cmd := parseCreatedBy(c.in)
		if k != c.kind || i != c.instr || cmd != c.cmd {
			t.Errorf("parseCreatedBy(%q) = %q|%q|%q, want %q|%q|%q", c.in, k, i, cmd, c.kind, c.instr, c.cmd)
		}
	}
}

func hintSteps(in ...[2]string) []model.ImageLayerStep {
	out := []model.ImageLayerStep{}
	for n, p := range in {
		out = append(out, model.ImageLayerStep{Index: n + 1, Kind: p[0], Command: p[1]})
	}
	return out
}

func TestLayerHints(t *testing.T) {
	h := layerHints(hintSteps(
		[2]string{"RUN", "apt-get update && apt-get install -y curl"},
		[2]string{"RUN", "apt-get update && apt-get install -y git && rm -rf /var/lib/apt/lists/*"},
		[2]string{"COPY", ". /app"},
		[2]string{"RUN", "npm ci"},
		[2]string{"ADD", "https://example.com/x.tgz /opt/"},
	))
	got := map[int]bool{}
	for _, x := range h {
		got[x.Step] = true
	}
	if len(h) != 3 || !got[1] || !got[3] || !got[5] || got[2] {
		t.Fatalf("unexpected hints: %+v", h)
	}
	if h := layerHints(hintSteps([2]string{"COPY", ". /app"}, [2]string{"CMD", "x"})); len(h) != 0 {
		t.Fatalf("no hint expected: %+v", h)
	}
	if h := layerHints(hintSteps([2]string{"COPY", "package.json ./"}, [2]string{"RUN", "npm ci"})); len(h) != 0 {
		t.Fatalf("no hint expected: %+v", h)
	}
}

func TestBuildLayersReport(t *testing.T) {
	r := buildLayersReport("sha256:x", 1000, []model.ImageLayer{
		{ID: "sha256:c", CreatedBy: `/bin/sh -c #(nop)  CMD ["x"]`, Size: 0},
		{ID: "<missing>", CreatedBy: "/bin/sh -c apt-get install -y a", Size: 300},
		{ID: "<missing>", CreatedBy: "/bin/sh -c #(nop) ADD file:1 in / ", Size: 100},
	})
	if len(r.Steps) != 3 || r.Steps[0].Kind != "ADD" || r.Steps[2].Kind != "CMD" || r.Steps[0].Index != 1 {
		t.Fatalf("order wrong: %+v", r.Steps)
	}
	if r.HistorySize != 400 || r.ContentSteps != 2 || r.LargestStep != 2 || r.Steps[1].Share != 0.75 {
		t.Fatalf("totals wrong: %+v", r)
	}
}

func TestBestSharedBase(t *testing.T) {
	mine := []string{"a", "b", "c"}
	b := bestSharedBase(mine, []baseCandidate{
		{ref: "z:1", layers: []string{"a", "b"}},
		{ref: "y:1", layers: []string{"a", "b", "x"}},
		{ref: "n:1", layers: []string{"q"}},
	})
	if b == nil || b.Count != 2 || b.Ref != "y:1" {
		t.Fatalf("got %+v", b)
	}
	if bestSharedBase(mine, []baseCandidate{{ref: "n", layers: []string{"q"}}}) != nil {
		t.Fatal("expected nil")
	}
}

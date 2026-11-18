package runspec

import (
	"strings"
	"testing"
)

func TestDockerRunDefaultsOmitted(t *testing.T) {
	got := ToDockerRun(Spec{Name: "x", Image: "nginx"}, Options{Shell: Bash})
	if got != "docker run -d \\\n  --name x \\\n  nginx\n" {
		t.Errorf("got %q", got)
	}
}

func TestDockerRunBash(t *testing.T) {
	got := ToDockerRun(richSpec(), Options{Shell: Bash})
	want := `docker run -d \
  --name web \
  -it \
  --restart on-failure:3 \
  --init \
  --privileged \
  --read-only \
  --user 1000:1000 \
  --workdir /srv \
  --hostname web-host \
  --entrypoint /entry.sh \
  --network appnet \
  --network-alias web-alias \
  -p 127.0.0.1:8080:80 \
  -p 53:53/udp \
  -v /host/data:/data:ro \
  -v pgdata:/var/lib/db \
  --tmpfs /run:size=65536 \
  -e MODE=prod \
  -e DB_PASSWORD="$DB_PASSWORD" \
  -e 'PRICE=$5' \
  --label team=core \
  --memory 512m \
  --memory-reservation 256m \
  --cpus 1.5 \
  --pids-limit 100 \
  --shm-size 128m \
  --cap-add NET_ADMIN \
  --cap-drop MKNOD \
  --add-host db:10.0.0.5 \
  --dns 1.1.1.1 \
  --log-driver local \
  --log-opt max-size=10m \
  --ulimit nofile=1024:2048 \
  --health-cmd 'curl -f http://localhost/ || exit 1' \
  --health-interval 30s \
  --health-timeout 5s \
  --health-retries 3 \
  --stop-signal SIGINT \
  --stop-timeout 20 \
  nginx:1.27 \
  --first serve --msg 'hello world'
docker network connect --alias o1 other web
# not included: annotations
`
	if got != want {
		t.Errorf("mismatch\n--- got\n%s\n--- want\n%s", got, want)
	}
}

func TestDockerRunPowerShell(t *testing.T) {
	got := ToDockerRun(richSpec(), Options{Shell: PowerShell})
	for _, want := range []string{"docker run -d `\n", `-e "DB_PASSWORD=$env:DB_PASSWORD" `, "-e 'PRICE=$5' `", "--msg 'hello world'", "--dns 1.1.1.1 `"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if strings.Contains(got, " \\\n") {
		t.Errorf("bash continuation in PowerShell output")
	}
}

func TestSecretsNeverInDefaultOutput(t *testing.T) {
	s := richSpec()
	for _, sh := range []Shell{Bash, PowerShell} {
		if out := ToDockerRun(s, Options{Shell: sh}); strings.Contains(out, "s3cr3t") {
			t.Errorf("%s default output leaks the secret:\n%s", sh, out)
		}
	}
	if out := ToCompose(s, Options{}); strings.Contains(out, "s3cr3t") {
		t.Errorf("compose default output leaks the secret:\n%s", out)
	}
	if out := ToDockerRun(s, Options{Shell: Bash, IncludeSecrets: true}); !strings.Contains(out, "-e 'DB_PASSWORD="+secretValue+"'") {
		t.Errorf("explicit flag must include the value:\n%s", out)
	}
	if got := s.SecretKeys(); len(got) != 1 || got[0] != "DB_PASSWORD" {
		t.Errorf("SecretKeys = %v", got)
	}
}

func TestDockerRunEntrypointCleared(t *testing.T) {
	got := ToDockerRun(Spec{Image: "x", EntrypointSet: true, Cmd: []string{"sh"}, CmdSet: true}, Options{Shell: Bash})
	if !strings.Contains(got, "--entrypoint '' \\\n  x \\\n  sh\n") {
		t.Errorf("got %q", got)
	}
}

func TestDockerRunHostNetworkAndDefaultBridgeExtras(t *testing.T) {
	got := ToDockerRun(Spec{Name: "h", Image: "x", NetworkMode: "host"}, Options{Shell: Bash})
	if !strings.Contains(got, "--network host") || strings.Contains(got, "network connect") {
		t.Errorf("got %q", got)
	}
	got = ToDockerRun(Spec{Name: "b", Image: "x", Networks: []NetAttach{{Name: "n1"}}}, Options{Shell: Bash})
	if !strings.Contains(got, "docker network connect n1 b\n") || strings.Contains(got, "--network ") {
		t.Errorf("got %q", got)
	}
}

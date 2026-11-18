package runspec

import (
	"time"

	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	"github.com/moby/moby/api/types/image"
)

const secretValue = "s3cr3t-Value!$x"

// richSpec exercises every flag the renderers support.
func richSpec() Spec {
	stop := 20
	return Spec{
		Name: "web", Image: "nginx:1.27", Stdin: true, TTY: true, Init: true, Privileged: true, ReadOnly: true,
		Restart: "on-failure:3", Hostname: "web-host", User: "1000:1000", WorkDir: "/srv", StopSignal: "SIGINT", StopTimeout: &stop,
		Entrypoint: []string{"/entry.sh", "--first"}, EntrypointSet: true, Cmd: []string{"serve", "--msg", "hello world"}, CmdSet: true,
		Env:    []EnvVar{{Key: "MODE", Value: "prod"}, {Key: "DB_PASSWORD", Value: secretValue, Secret: true}, {Key: "PRICE", Value: "$5"}},
		Labels: []KV{{"team", "core"}},
		Ports:  []Port{{HostIP: "127.0.0.1", HostPort: "8080", ContainerPort: 80, Proto: "tcp"}, {HostPort: "53", ContainerPort: 53, Proto: "udp"}},
		Mounts: []Mount{
			{Type: "bind", Source: "/host/data", Target: "/data", ReadOnly: true},
			{Type: "volume", Source: "pgdata", Target: "/var/lib/db"},
			{Type: "tmpfs", Target: "/run", TmpfsOpts: "size=65536"},
		},
		NetworkMode: "appnet",
		Networks:    []NetAttach{{Name: "appnet", Aliases: []string{"web-alias"}}, {Name: "other", Aliases: []string{"o1"}}},
		Health:      &Health{Test: []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"}, Interval: 30 * time.Second, Timeout: 5 * time.Second, Retries: 3},
		MemoryBytes: 512 << 20, MemoryReservation: 256 << 20, NanoCPUs: 1_500_000_000, ShmSize: 128 << 20, PidsLimit: 100,
		CapAdd: []string{"NET_ADMIN"}, CapDrop: []string{"MKNOD"}, ExtraHosts: []string{"db:10.0.0.5"}, DNS: []string{"1.1.1.1"},
		Ulimits: []string{"nofile=1024:2048"}, LogDriver: "local", LogOpts: []KV{{"max-size", "10m"}},
		NotIncluded: []string{"annotations"},
	}
}

// withImageConfig sets the image defaults a container is compared against.
func withImageConfig(r *image.InspectResponse, env, cmd []string, labels map[string]string, volume string) *image.InspectResponse {
	r.Config = &dockerspec.DockerOCIImageConfig{}
	r.Config.Env, r.Config.Cmd, r.Config.Labels = env, cmd, labels
	if volume != "" {
		r.Config.Volumes = map[string]struct{}{volume: {}}
	}
	return r
}

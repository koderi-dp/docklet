package runspec

import (
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ComposeFile is a whole Compose document made from containers.
type ComposeFile struct {
	Text string
	// NotIncluded lists what could not be carried over, each entry prefixed "<service>: ".
	NotIncluded []string
	// SecretKeys lists the secret-looking variables as "<service>.<KEY>".
	SecretKeys []string
}

// keyTable hands out top-level keys so that one key never stands for two different resources.
type keyTable map[string]string // key -> the resource it stands for

func (t keyTable) key(key, actual string) string {
	for n := 1; ; n++ {
		k := key
		if n > 1 {
			k = key + "-" + strconv.Itoa(n)
		}
		if cur, taken := t[k]; !taken || cur == actual {
			t[k] = actual
			return k
		}
	}
}

// ToComposeFile renders specs as one Compose document for a project named project.
//
// A container that does not already belong to a Compose project of that name keeps pointing at
// its existing volumes and networks (by name, external), so its data is not left behind.
// References to containers that are not among specs are left out and reported.
func ToComposeFile(project string, specs []Spec, o Options) ComposeFile {
	out := ComposeFile{NotIncluded: []string{}, SecretKeys: []string{}}

	// Service names, unique in the file, and where each container's name leads.
	names := make([]string, len(specs))
	used := keyTable{}
	byContainer := map[string]string{}
	for i, s := range specs {
		names[i] = used.key(s.serviceName(), "#"+strconv.Itoa(i))
		if s.Name != "" {
			if _, dup := byContainer[s.Name]; !dup {
				byContainer[s.Name] = names[i]
			}
		}
	}

	secretUses := map[string]int{}
	for _, s := range specs {
		for _, k := range s.SecretKeys() {
			secretUses[k]++
		}
	}

	root := mapNode()
	put(root, "name", project)
	services := mapNode()
	topVolumes, topNetworks := mapNode(), mapNode()
	vols, nets := keyTable{}, keyTable{}

	for i, s := range specs {
		name := names[i]
		note := func(text string) { out.NotIncluded = append(out.NotIncluded, name+": "+text) }
		for _, k := range s.SecretKeys() {
			out.SecretKeys = append(out.SecretKeys, name+"."+k)
		}
		if s.ComposeProject != project {
			s.ComposeProject = "" // rendered as a container outside the project: existing resources are reused
		}
		s = rewriteRefs(s, byContainer, note)

		svc, tv, tn := buildService(s, renderCtx{
			o: o,
			secretVar: func(key string) string {
				if secretUses[key] > 1 {
					return secretVarName(strings.ToUpper(name) + "_" + key)
				}
				return secretVarName(key)
			},
			volKey: vols.key,
			netKey: nets.key,
		})
		put(services, name, svc)
		mergeTop(topVolumes, tv)
		mergeTop(topNetworks, tn)
		for _, n := range composeNotIncluded(s) {
			note(n)
		}
	}

	if len(services.Content) == 0 {
		services.Style = yaml.FlowStyle
	}
	put(root, "services", services)
	if len(topVolumes.Content) > 0 {
		put(root, "volumes", topVolumes)
	}
	if len(topNetworks.Content) > 0 {
		put(root, "networks", topNetworks)
	}
	out.Text = encodeCompose(root, out.NotIncluded)
	return out
}

// mergeTop adds the definitions of one service to the file's, each key once.
func mergeTop(dst, src *yaml.Node) {
	if src == nil {
		return
	}
	for i := 0; i+1 < len(src.Content); i += 2 {
		if !hasKey(dst, src.Content[i].Value) {
			dst.Content = append(dst.Content, src.Content[i], src.Content[i+1])
		}
	}
}

// rewriteRefs points links, volumes-from and a container network mode at services of the file,
// and drops (with a note) the ones whose target is not in it.
func rewriteRefs(s Spec, byContainer map[string]string, note func(string)) Spec {
	keep := func(items []string, label string) []string {
		var out []string
		for _, it := range items {
			target, rest, _ := strings.Cut(it, ":")
			svc, ok := byContainer[strings.TrimPrefix(target, "/")]
			if !ok {
				note(label + " " + target + " (not in this file)")
				continue
			}
			if rest != "" {
				svc += ":" + rest
			}
			out = append(out, svc)
		}
		return out
	}
	s.Links = keep(s.Links, "link")
	s.VolumesFrom = keep(s.VolumesFrom, "volumes from")
	if target, ok := strings.CutPrefix(s.NetworkMode, "container:"); ok {
		if svc, found := byContainer[target]; found {
			s.NetworkMode = "service:" + svc
		} else {
			note("network mode " + s.NetworkMode + " (not in this file)")
			s.NetworkMode = ""
		}
	}
	return s
}

package compose

import (
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"

	"docklet/internal/model"

	"gopkg.in/yaml.v3"
)

var yamlLineRe = regexp.MustCompile(`^(?:yaml: )?(?:unmarshal errors:\s*)?line (\d+): `)

// Outline lists the services of a Compose file with the line of each key. It reads
// every YAML document and never reformats anything.
func Outline(text string) model.ComposeFileOutline {
	out := model.ComposeFileOutline{Services: []model.ComposeOutlineItem{}}
	dec := yaml.NewDecoder(strings.NewReader(text))
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			return model.ComposeFileOutline{
				Services: []model.ComposeOutlineItem{},
				Error:    yamlError(err),
			}
		}
		out.Services = append(out.Services, servicesOf(&doc)...)
	}
}

func servicesOf(doc *yaml.Node) []model.ComposeOutlineItem {
	root := doc
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		root = root.Content[0]
	}
	root = resolveAlias(root)
	if root == nil || root.Kind != yaml.MappingNode {
		return nil
	}
	var items []model.ComposeOutlineItem
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != "services" {
			continue
		}
		svc := resolveAlias(root.Content[i+1])
		if svc == nil || svc.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j+1 < len(svc.Content); j += 2 {
			k := svc.Content[j]
			items = append(items, model.ComposeOutlineItem{Name: k.Value, Line: k.Line})
		}
	}
	return items
}

func resolveAlias(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
}

func yamlError(err error) *model.ComposeYAMLError {
	msg := strings.TrimSpace(err.Error())
	line := 1
	if m := yamlLineRe.FindStringSubmatch(msg); m != nil {
		if n, e := strconv.Atoi(m[1]); e == nil && n > 0 {
			line = n
		}
		msg = msg[len(m[0]):]
	} else {
		msg = strings.TrimPrefix(msg, "yaml: ")
	}
	return &model.ComposeYAMLError{Line: line, Message: msg}
}

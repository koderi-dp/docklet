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
				Error:    yamlError(err, text),
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

// parserErrorPrefixes start the messages of yaml.v3 parser errors (parserc.go). The library
// prints the zero-based mark line for those but adds one only to scanner errors, so their
// printed line is one too low.
var parserErrorPrefixes = []string{
	"did not find expected ", "found undefined tag handle", "found duplicate %", "found incompatible YAML",
}

// yamlError turns a yaml.v3 error into a message and the 1-based line of the mistake.
// Known limit: for "did not find expected key" and "did not find expected '-' indicator" the
// library only reports where the surrounding collection starts, so that line is shown.
func yamlError(err error, text string) *model.ComposeYAMLError {
	msg := strings.TrimSpace(err.Error())
	line := 1
	if m := yamlLineRe.FindStringSubmatch(msg); m != nil {
		if n, e := strconv.Atoi(m[1]); e == nil && n > 0 {
			line = n
		}
		msg = msg[len(m[0]):]
		for _, p := range parserErrorPrefixes {
			if strings.HasPrefix(msg, p) {
				line++
				break
			}
		}
	} else {
		msg = strings.TrimPrefix(msg, "yaml: ")
	}
	if strings.HasPrefix(msg, "found a tab character that violates indentation") {
		line = tabLine(text, line)
	}
	return &model.ComposeYAMLError{Line: line, Message: msg}
}

// tabLine finds the first line from `from` (1-based) whose indentation holds a tab. The
// library reports where the plain scalar before the tab started, not the tab itself.
func tabLine(text string, from int) int {
	lines := strings.Split(text, "\n")
	for i := from - 1; i >= 0 && i < len(lines); i++ {
		indent := lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " \t"))]
		if strings.Contains(indent, "\t") {
			return i + 1
		}
	}
	return from
}

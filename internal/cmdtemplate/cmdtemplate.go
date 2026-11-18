// Package cmdtemplate handles user-written command templates such as "alacritty -e {cmd}"
// or "myeditor {path}": split into argv (double quotes group words, no shell is involved)
// with one placeholder replaced by caller-supplied values.
package cmdtemplate

import (
	"errors"
	"fmt"
	"strings"
)

// Validate checks that the template contains the placeholder and names a program.
func Validate(tpl, placeholder string) error {
	if !strings.Contains(tpl, placeholder) {
		return fmt.Errorf("the custom command must contain %s", placeholder)
	}
	fields := Split(tpl)
	if len(fields) == 0 {
		return errors.New("the custom command is empty")
	}
	if strings.Contains(fields[0], placeholder) {
		return fmt.Errorf("%s cannot be part of the program name", placeholder)
	}
	return nil
}

// Expand splits the template into argv and replaces a lone placeholder word by values;
// a placeholder inside a longer word becomes the values joined by spaces.
func Expand(tpl, placeholder string, values []string) ([]string, error) {
	if err := Validate(tpl, placeholder); err != nil {
		return nil, err
	}
	var out []string
	for _, f := range Split(tpl) {
		switch {
		case f == placeholder:
			out = append(out, values...)
		case strings.Contains(f, placeholder):
			out = append(out, strings.ReplaceAll(f, placeholder, strings.Join(values, " ")))
		default:
			out = append(out, f)
		}
	}
	return out, nil
}

// Split breaks s on spaces and tabs; double quotes group words.
func Split(s string) []string {
	var out []string
	var cur strings.Builder
	inQ, has := false, false
	for _, r := range s {
		switch {
		case r == '"':
			inQ, has = !inQ, true
		case (r == ' ' || r == '\t') && !inQ:
			if has {
				out = append(out, cur.String())
				cur.Reset()
				has = false
			}
		default:
			cur.WriteRune(r)
			has = true
		}
	}
	if has {
		out = append(out, cur.String())
	}
	return out
}

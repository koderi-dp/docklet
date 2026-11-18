package services

import (
	"fmt"
	"regexp"
	"strings"
)

var envNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// scriptExt is the extension of the launch script for the OS.
func scriptExt(goos string) string {
	switch goos {
	case "windows":
		return ".cmd"
	case "darwin":
		return ".command" // opened by Terminal.app
	}
	return ".sh"
}

// buildScript renders the launch script: environment, the docker/ssh/wsl command, and a pause
// on failure so an error (stopped container, no shell) stays readable. Everything embedded is
// validated or quoted; values that cannot be quoted safely are rejected.
func buildScript(goos string, tc termCommand) (string, error) {
	for _, kv := range tc.Set {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || !envNameRe.MatchString(k) || strings.ContainsAny(v, "\x00\r\n") {
			return "", fmt.Errorf("unsupported environment entry for the terminal: %q", k)
		}
	}
	for _, k := range tc.Unset {
		if !envNameRe.MatchString(k) {
			return "", fmt.Errorf("invalid environment name %q", k)
		}
	}
	for _, a := range tc.Argv {
		if a == "" || strings.ContainsAny(a, "\x00\r\n") {
			return "", fmt.Errorf("unsupported argument for the terminal: %q", a)
		}
	}
	if goos == "windows" {
		return windowsScript(tc)
	}
	return posixScript(tc), nil
}

func windowsScript(tc termCommand) (string, error) {
	bad := func(s string) bool { return strings.ContainsAny(s, `"%`) }
	var b strings.Builder
	b.WriteString("@echo off\r\n")
	for _, k := range tc.Unset {
		fmt.Fprintf(&b, "set \"%s=\"\r\n", k)
	}
	for _, kv := range tc.Set {
		if bad(kv) {
			return "", fmt.Errorf("the connection settings contain characters that cannot be passed to cmd.exe safely: %q", kv)
		}
		fmt.Fprintf(&b, "set \"%s\"\r\n", kv)
	}
	parts := make([]string, len(tc.Argv))
	for i, a := range tc.Argv {
		if bad(a) {
			return "", fmt.Errorf("argument contains characters that cannot be passed to cmd.exe safely: %q", a)
		}
		parts[i] = cmdQuote(a)
	}
	b.WriteString(strings.Join(parts, " ") + "\r\nif errorlevel 1 pause\r\n")
	return b.String(), nil
}

// cmdQuote wraps an argument in double quotes when it has characters special to cmd.exe.
func cmdQuote(a string) string {
	if strings.ContainsAny(a, " \t&|<>^()") {
		return `"` + a + `"`
	}
	return a
}

func posixScript(tc termCommand) string {
	var b strings.Builder
	b.WriteString("#!/bin/sh\nrm -f -- \"$0\"\n")
	for _, k := range tc.Unset {
		fmt.Fprintf(&b, "unset %s\n", k)
	}
	for _, kv := range tc.Set {
		k, v, _ := strings.Cut(kv, "=")
		fmt.Fprintf(&b, "export %s=%s\n", k, shQuote(v))
	}
	parts := make([]string, len(tc.Argv))
	for i, a := range tc.Argv {
		parts[i] = shQuote(a)
	}
	b.WriteString(strings.Join(parts, " ") + "\nst=$?\n")
	b.WriteString("if [ \"$st\" -ne 0 ]; then printf '\\nExited with status %s. Press Enter to close...' \"$st\"; read _dummy; fi\n")
	return b.String()
}

// shQuote single-quotes a string for POSIX sh.
func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

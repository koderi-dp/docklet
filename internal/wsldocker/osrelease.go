// Package wsldocker detects what a WSL distribution has for Docker and installs
// Docker Engine into it by running a fixed list of commands through wsl.exe.
package wsldocker

import "strings"

// OSRelease is the subset of /etc/os-release that decides how Docker is installed.
type OSRelease struct {
	ID              string
	IDLike          []string
	VersionID       string
	VersionCodename string
	UbuntuCodename  string
	PrettyName      string
}

// ParseOSRelease reads os-release(5) text: KEY=value lines, values optionally
// quoted, comments and unknown keys ignored. Missing fields stay empty.
func ParseOSRelease(text string) OSRelease {
	var o OSRelease
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		line = strings.TrimSpace(line)
		k, v, ok := strings.Cut(line, "=")
		if !ok || strings.HasPrefix(line, "#") {
			continue
		}
		v = unquote(strings.TrimSpace(v))
		switch strings.TrimSpace(k) {
		case "ID":
			o.ID = strings.ToLower(v)
		case "ID_LIKE":
			o.IDLike = strings.Fields(strings.ToLower(v))
		case "VERSION_ID":
			o.VersionID = v
		case "VERSION_CODENAME":
			o.VersionCodename = v
		case "UBUNTU_CODENAME":
			o.UbuntuCodename = v
		case "PRETTY_NAME":
			o.PrettyName = v
		}
	}
	return o
}

func unquote(v string) string {
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		return v[1 : len(v)-1]
	}
	return v
}

// Codename is the apt suite name Docker's repository uses.
func (o OSRelease) Codename() string {
	if o.UbuntuCodename != "" {
		return o.UbuntuCodename
	}
	return o.VersionCodename
}

// Family names the Docker install guide that applies, or "" when none is known.
func (o OSRelease) Family() string {
	ids := append([]string{o.ID}, o.IDLike...)
	for _, id := range ids {
		switch id {
		case "ubuntu", "debian", "fedora", "rhel", "centos", "sles":
			return id
		case "opensuse", "opensuse-leap", "suse":
			return "sles"
		}
	}
	return ""
}

// DocsURL is Docker's official install page for this distribution family.
func (o OSRelease) DocsURL() string {
	if f := o.Family(); f != "" {
		return "https://docs.docker.com/engine/install/" + f + "/"
	}
	return "https://docs.docker.com/engine/install/"
}

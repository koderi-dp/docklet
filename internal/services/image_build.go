package services

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"docklet/internal/model"
	"docklet/internal/registryauth"
)

var (
	stageNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	platformRe  = regexp.MustCompile(`^[a-z0-9]+/[a-z0-9]+(/[a-z0-9]+)?$`)
)

// validatedBuild is a BuildSpec after validation, with resolved paths.
type validatedBuild struct {
	ContextDir string
	Dockerfile string // slash-separated, relative to ContextDir
	Tags       []string
	BuildArgs  map[string]*string
	Target     string
	Platform   *ocispec.Platform
	NoCache    bool
	Pull       bool
}

// validateBuildSpec checks the spec and the local files it points at. The
// Dockerfile must lie inside the context directory.
func validateBuildSpec(spec model.BuildSpec) (validatedBuild, error) {
	var v validatedBuild
	dir := strings.TrimSpace(spec.ContextDir)
	if dir == "" {
		return v, errors.New("choose a build context directory")
	}
	if !filepath.IsAbs(dir) {
		return v, errors.New("the build context must be an absolute path")
	}
	dir = filepath.Clean(dir)
	st, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return v, fmt.Errorf("build context %q does not exist", dir)
		}
		return v, fmt.Errorf("cannot read build context: %w", err)
	}
	if !st.IsDir() {
		return v, fmt.Errorf("build context %q is not a directory", dir)
	}
	v.ContextDir = dir

	df := strings.TrimSpace(spec.Dockerfile)
	if df == "" {
		df = "Dockerfile"
	}
	if filepath.IsAbs(df) || strings.HasPrefix(df, "/") || strings.HasPrefix(df, `\`) || filepath.VolumeName(df) != "" {
		return v, errors.New("the Dockerfile path must be relative to the build context")
	}
	rel := path.Clean(filepath.ToSlash(df))
	if rel == ".." || strings.HasPrefix(rel, "../") || rel == "." {
		return v, errors.New("the Dockerfile must be inside the build context")
	}
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if r, err := filepath.Rel(dir, full); err != nil || strings.HasPrefix(filepath.ToSlash(r), "../") {
		return v, errors.New("the Dockerfile must be inside the build context")
	}
	fi, err := os.Stat(full)
	if err != nil {
		if os.IsNotExist(err) {
			return v, fmt.Errorf("Dockerfile %q was not found in the build context", rel)
		}
		return v, fmt.Errorf("cannot read Dockerfile: %w", err)
	}
	if fi.IsDir() {
		return v, fmt.Errorf("%q is a directory, not a Dockerfile", rel)
	}
	v.Dockerfile = rel

	seen := map[string]bool{}
	for _, raw := range spec.Tags {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		t, err := validateTarget(raw)
		if err != nil {
			return v, err
		}
		if !seen[t] {
			seen[t] = true
			v.Tags = append(v.Tags, t)
		}
	}

	if len(spec.BuildArgs) > 0 {
		v.BuildArgs = map[string]*string{}
		for k, val := range spec.BuildArgs {
			k = strings.TrimSpace(k)
			if !envKeyRe.MatchString(k) {
				return v, fmt.Errorf("invalid build argument name %q", k)
			}
			val := val
			v.BuildArgs[k] = &val
		}
	}

	if t := strings.TrimSpace(spec.Target); t != "" {
		if !stageNameRe.MatchString(t) {
			return v, fmt.Errorf("invalid target stage %q", t)
		}
		v.Target = t
	}
	if p := strings.TrimSpace(spec.Platform); p != "" {
		if !platformRe.MatchString(p) {
			return v, fmt.Errorf("invalid platform %q (expected os/arch[/variant], e.g. linux/amd64)", p)
		}
		parts := strings.Split(p, "/")
		pl := &ocispec.Platform{OS: parts[0], Architecture: parts[1]}
		if len(parts) == 3 {
			pl.Variant = parts[2]
		}
		v.Platform = pl
	}
	v.NoCache, v.Pull = spec.NoCache, spec.Pull
	return v, nil
}

// fromRegistries returns the registry hosts of the base images in a Dockerfile.
var fromRe = regexp.MustCompile(`(?im)^\s*FROM\s+(?:--\S+\s+)*(\S+)`)

func baseImageRegistries(dockerfile []byte) []string {
	stages := map[string]bool{}
	seen := map[string]bool{}
	var out []string
	for _, m := range fromRe.FindAllSubmatch(dockerfile, -1) {
		img := string(m[1])
		if stages[strings.ToLower(img)] || strings.EqualFold(img, "scratch") || strings.Contains(img, "$") {
			continue
		}
		host, err := registryauth.RegistryOf(img)
		if err != nil || seen[host] {
			continue
		}
		seen[host] = true
		out = append(out, host)
	}
	return out
}

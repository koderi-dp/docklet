package services

import (
	"context"
	"fmt"
	"strings"

	"docklet/internal/model"
	"docklet/internal/registryauth"
)

// resolver returns the credential resolver (default: Docker CLI config on this machine).
func (s *ImageService) resolver() *registryauth.Resolver {
	if s.Registry != nil {
		return s.Registry
	}
	return &registryauth.Resolver{}
}

// RegistryAuthStatus reports whether Docker CLI credentials exist for the
// registry of ref. Credentials are read from this computer's Docker config and
// credential helpers; nothing secret is ever returned. connectionID is accepted
// for API consistency (credentials do not depend on the Engine).
func (s *ImageService) RegistryAuthStatus(ctx context.Context, connectionID, ref string) (model.RegistryAuthStatus, error) {
	host, err := registryauth.RegistryOf(ref)
	if err != nil {
		return model.RegistryAuthStatus{}, err
	}
	st := model.RegistryAuthStatus{Registry: host, Source: registryauth.SourceNone}
	creds, rerr := s.resolver().ResolveHost(ctx, host)
	if rerr == nil && creds.Has() {
		st.HasCredentials, st.Source = true, creds.Source
	}
	return st, nil
}

// authFor resolves credentials for ref. A resolution error is not fatal: it is
// logged (redacted) and the operation continues without credentials.
func (s *ImageService) authFor(ctx context.Context, ref string, h pullSink) registryauth.Credentials {
	creds, err := s.resolver().Resolve(ctx, ref)
	if err != nil {
		h.Log("Warning: " + registryauth.Redact(err.Error()) + "; continuing without credentials")
		return registryauth.Credentials{}
	}
	if creds.Has() {
		h.Log(fmt.Sprintf("Using Docker credentials for %s (%s)", creds.Registry, creds.Source))
	}
	return creds
}

func isAuthFailure(err error) bool {
	low := strings.ToLower(err.Error())
	return strings.Contains(low, "unauthorized") || strings.Contains(low, "denied") ||
		strings.Contains(low, "authentication required") || strings.Contains(low, "no basic auth credentials")
}

// registryAuthError makes a registry authorization failure actionable.
// withEngineMsg appends the Engine's own message when no credentials exist
// (a pull of a missing public image is also reported as "denied").
func registryAuthError(err error, creds registryauth.Credentials, ref string, withEngineMsg bool) error {
	if err == nil || !isAuthFailure(err) {
		return err
	}
	host, herr := registryauth.RegistryOf(ref)
	if herr != nil {
		host = creds.Registry
	}
	if !creds.Has() {
		if withEngineMsg {
			return fmt.Errorf("No credentials for %s. Run `docker login %s` in a terminal, then retry. The registry said: %w", host, host, err)
		}
		return fmt.Errorf("No credentials for %s. Run `docker login %s` in a terminal, then retry.", host, host)
	}
	return fmt.Errorf("%s rejected the saved credentials (%s). Run `docker login %s` in a terminal to refresh them, then retry. Details: %w",
		host, creds.Source, host, err)
}

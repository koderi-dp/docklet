package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moby/moby/client"

	"docklet/internal/engine"
	"docklet/internal/model"
	"docklet/internal/registryauth"
)

// RegistryService signs in and out of image registries. Secrets are verified
// through the Engine, saved on this computer (credential helper or the OS
// credential store) and never returned to the frontend.
type RegistryService struct {
	Engines engine.Provider
	// Auth reads and writes the credential stores (nil = this computer's Docker config + OS credential store).
	Auth *registryauth.Resolver
	// Login verifies credentials with the Engine; tests replace it.
	Login func(ctx context.Context, cli *client.Client, serverAddress, username, secret string) (identityToken string, err error)
}

func (s *RegistryService) auth() *registryauth.Resolver {
	if s.Auth != nil {
		return s.Auth
	}
	return registryauth.DefaultResolver()
}

// List returns the registries the user is signed in to (host, user name and
// where the secret is kept).
func (s *RegistryService) List(ctx context.Context) (model.RegistryList, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	entries, warns := s.auth().List(ctx)
	out := model.RegistryList{Entries: make([]model.RegistryEntry, 0, len(entries)), Warnings: warns}
	if out.Warnings == nil {
		out.Warnings = []string{}
	}
	for _, e := range entries {
		out.Entries = append(out.Entries, toRegistryEntry(e))
	}
	return out, nil
}

func toRegistryEntry(e registryauth.Entry) model.RegistryEntry {
	return model.RegistryEntry{Host: e.Host, Username: e.Username, Store: e.Kind, StoreLabel: e.Label}
}

// StorageFor says where a new sign-in for host would be kept.
func (s *RegistryService) StorageFor(host string) (string, error) {
	h, err := registryauth.CleanHost(host)
	if err != nil {
		return "", err
	}
	return s.auth().StorageLabel(h), nil
}

// HostOf returns the registry host of an image reference ("docker.io" for Docker Hub).
func (s *RegistryService) HostOf(ref string) (string, error) { return registryauth.RegistryOf(ref) }

// SignIn verifies the credentials with the Engine of connectionID and, only on
// success, stores them. The Engine's own error text is returned on failure.
func (s *RegistryService) SignIn(ctx context.Context, connectionID, host, username, secret string) (model.RegistryEntry, error) {
	h, err := registryauth.CleanHost(host)
	if err != nil {
		return model.RegistryEntry{}, err
	}
	username = strings.TrimSpace(username)
	if username == "" || secret == "" {
		return model.RegistryEntry{}, errors.New("enter a user name and a password or access token")
	}
	if s.Engines == nil {
		return model.RegistryEntry{}, errors.New("no engine provider configured")
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	cli, err := s.Engines.Client(ctx, connectionID)
	if err != nil || cli == nil {
		return model.RegistryEntry{}, errors.New("connect to an engine first: Docklet verifies the credentials through it")
	}
	login := s.Login
	if login == nil {
		login = engineLogin
	}
	token, err := login(ctx, cli, registryauth.ServerKey(h), username, secret)
	if err != nil {
		return model.RegistryEntry{}, scrubSecret(err, secret)
	}
	if token != "" {
		username, secret = "<token>", token
	}
	e, err := s.auth().Save(ctx, h, username, secret)
	if err != nil {
		return model.RegistryEntry{}, scrubSecret(err, secret)
	}
	return toRegistryEntry(e), nil
}

// SignOut removes the sign-in of host from the store it lives in (store is the value List returned).
func (s *RegistryService) SignOut(ctx context.Context, host, store string) error {
	h, err := registryauth.CleanHost(host)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return s.auth().Remove(ctx, h, store)
}

func engineLogin(ctx context.Context, cli *client.Client, serverAddress, username, secret string) (string, error) {
	res, err := cli.RegistryLogin(ctx, client.RegistryLoginOptions{Username: username, Password: secret, ServerAddress: serverAddress})
	if err != nil {
		return "", err
	}
	return res.Auth.IdentityToken, nil
}

// scrubSecret keeps the registry's message but guarantees the secret is not in it.
func scrubSecret(err error, secret string) error {
	msg := err.Error()
	if secret != "" {
		msg = strings.ReplaceAll(msg, secret, "[redacted]")
	}
	return errors.New(msg)
}

package services

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/distribution/reference"
)

var imageIDRe = regexp.MustCompile(`^(sha256:)?[a-f0-9]{12,64}$`)

// validateRepoTag validates a user-supplied repository[:tag] (or @digest)
// reference and returns it trimmed. It never touches a shell; the Engine API
// receives the string as data, but the SDK concatenates it into URL paths, so
// rejecting anything that is not a well-formed reference matters.
func validateRepoTag(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", errors.New("image reference is empty")
	}
	if len(ref) > 255 {
		return "", errors.New("image reference is too long")
	}
	if imageIDRe.MatchString(ref) {
		return "", errors.New("an image ID is not a repository reference")
	}
	if _, err := reference.ParseNormalizedNamed(ref); err != nil {
		return "", fmt.Errorf("invalid image reference %q: %s", ref, refReason(err))
	}
	return ref, nil
}

// validateTarget validates the target of a tag operation: a name with an
// optional tag, never a digest.
func validateTarget(ref string) (string, error) {
	ref, err := validateRepoTag(ref)
	if err != nil {
		return "", err
	}
	n, _ := reference.ParseNormalizedNamed(ref)
	if _, ok := n.(reference.Digested); ok {
		return "", errors.New("a tag cannot be a digest reference")
	}
	return ref, nil
}

// validateImageRef accepts an existing image's identifier: an ID (short or
// full, optionally sha256:-prefixed) or a reference.
func validateImageRef(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if imageIDRe.MatchString(ref) {
		return ref, nil
	}
	return validateRepoTag(ref)
}

func refReason(err error) string {
	switch {
	case errors.Is(err, reference.ErrNameContainsUppercase):
		return "repository names must be lowercase"
	case errors.Is(err, reference.ErrTagInvalidFormat):
		return "invalid tag format"
	case errors.Is(err, reference.ErrReferenceInvalidFormat):
		return "invalid format (expected [registry/]name[:tag])"
	case errors.Is(err, reference.ErrNameTooLong):
		return "name is too long"
	case errors.Is(err, reference.ErrDigestInvalidFormat):
		return "invalid digest"
	}
	return err.Error()
}

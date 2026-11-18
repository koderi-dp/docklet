package model

// ImageUpdateStatus is the result of comparing a local image with its registry tag.
// UpdateAvailable is only meaningful when Error is empty.
type ImageUpdateStatus struct {
	Ref             string `json:"ref"`
	LocalDigest     string `json:"localDigest,omitempty"`  // digest(s) the local copy was pulled as, matching this repository
	RemoteDigest    string `json:"remoteDigest,omitempty"` // digest of the tag in the registry now
	UpdateAvailable bool   `json:"updateAvailable"`
	CheckedAt       int64  `json:"checkedAt"` // unix ms
	// Error explains why no verdict exists: not from a registry, credentials missing, offline...
	Error string `json:"error,omitempty"`
}

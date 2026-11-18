package model

// RegistryEntry is one registry sign-in. It never carries a password or token.
type RegistryEntry struct {
	Host     string `json:"host"`     // normalized; Docker Hub is "docker.io"
	Username string `json:"username"` // "<token>" when an identity token is stored
	// Store is "helper", "docklet" or "config".
	Store      string `json:"store"`
	StoreLabel string `json:"storeLabel"` // e.g. "Windows Credential Manager via docker-credential-wincred"
}

// RegistryList is the set of sign-ins Docklet can use, plus sources that could not be read.
type RegistryList struct {
	Entries  []RegistryEntry `json:"entries"`
	Warnings []string        `json:"warnings"`
}

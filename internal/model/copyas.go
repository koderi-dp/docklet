package model

// CopyAsResult is generated text for "Copy as…" (docker run command or Compose service).
type CopyAsResult struct {
	Text string `json:"text"`
	// SecretKeys are the secret-looking variables: replaced by a variable reference unless the caller
	// asked for the real values, in which case they are the ones exposed in Text.
	SecretKeys []string `json:"secretKeys"`
	// NotIncluded lists container settings that could not be expressed.
	NotIncluded []string `json:"notIncluded"`
}

// CopyAsInfo tells the UI which "Copy as" entries apply to a container.
type CopyAsInfo struct {
	// ConnectionKind is "" unless the container is a recognised database (postgres, mysql, ...).
	ConnectionKind  string   `json:"connectionKind"`
	ConnectionTitle string   `json:"connectionTitle"`
	SecretKeys      []string `json:"secretKeys"`
}

// ConnectionEntry is one copyable connection line.
type ConnectionEntry struct {
	Label        string `json:"label"`
	Text         string `json:"text"`
	UsesPassword bool   `json:"usesPassword"`
	Internal     bool   `json:"internal"`
}

// ConnectionStrings are the connection strings of a database container.
type ConnectionStrings struct {
	Kind        string            `json:"kind"`
	Title       string            `json:"title"`
	Host        string            `json:"host"`
	Reachable   bool              `json:"reachable"`
	Note        string            `json:"note"`
	PasswordEnv string            `json:"passwordEnv"`
	PasswordIn  bool              `json:"passwordIncluded"`
	Entries     []ConnectionEntry `json:"entries"`
}

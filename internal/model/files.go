package model

// FileEntry is one item of a container or volume directory listing.
type FileEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"` // absolute POSIX path inside the container (or volume root "/")
	IsDir      bool   `json:"isDir"`
	IsSymlink  bool   `json:"isSymlink"`
	LinkTarget string `json:"linkTarget,omitempty"`
	Size       int64  `json:"size"`
	Mode       string `json:"mode"`    // e.g. "drwxr-xr-x"
	ModTime    int64  `json:"modTime"` // unix ms, 0 = unknown
}

// FileListing is the direct children of a directory.
type FileListing struct {
	Path      string      `json:"path"`
	Entries   []FileEntry `json:"entries"`
	Truncated bool        `json:"truncated"` // more entries exist than Limit
	Limit     int         `json:"limit"`
	// Method is "exec" (fast, running container), "archive" (one directory through the archive API)
	// or "index" (served from a cached full-tree index of a stopped container).
	Method string `json:"method"`
	// IndexedAt (unix ms) and IndexedEntries describe the index behind a Method=="index" listing;
	// both are 0 for other methods. The UI can offer FileService.RefreshIndex next to them.
	IndexedAt      int64 `json:"indexedAt"`
	IndexedEntries int   `json:"indexedEntries"`
}

// FileIndexInfo describes a freshly built container file index (FileService.RefreshIndex).
type FileIndexInfo struct {
	Entries    int   `json:"entries"`
	IndexedAt  int64 `json:"indexedAt"` // unix ms
	DurationMs int64 `json:"durationMs"`
}

// FileText is an inline preview of a small text file.
type FileText struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Size      int64  `json:"size"`      // real file size
	Truncated bool   `json:"truncated"` // Content is only the first part
}

// FileCapabilities tells the UI which actions are possible and why not.
type FileCapabilities struct {
	CanWrite      bool   `json:"canWrite"` // upload / new folder
	WriteReason   string `json:"writeReason,omitempty"`
	CanDelete     bool   `json:"canDelete"`
	DeleteReason  string `json:"deleteReason,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
	Running       bool   `json:"running"`
}

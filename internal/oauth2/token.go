package oauth2

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const (
	// AuthorizationURL is the Atlassian OAuth2 authorization endpoint.
	AuthorizationURL = "https://auth.atlassian.com/authorize"

	// TokenURL is the Atlassian OAuth2 token endpoint.
	TokenURL = "https://auth.atlassian.com/oauth/token"

	// ResourcesURL is the Atlassian endpoint for listing accessible resources.
	ResourcesURL = "https://api.atlassian.com/oauth/token/accessible-resources"
)

// Token represents an OAuth2 access token response with metadata.
type Token struct {
	AccessToken  string    `json:"access_token"` // #nosec G117 -- intentionally serialized for token cache storage
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Scope        string    `json:"scope,omitempty"`
	ObtainedAt   time.Time `json:"obtained_at"`
	CloudID      string    `json:"cloud_id,omitempty"`
}

// Expired reports whether the token has expired or will expire within the
// given margin duration.
func (t *Token) Expired(margin time.Duration) bool {
	expiry := t.ObtainedAt.Add(time.Duration(t.ExpiresIn) * time.Second)
	return time.Now().After(expiry.Add(-margin))
}

// FileStore persists tokens as JSON files in a directory, one file per profile.
type FileStore struct {
	dir     string
	profile string
}

// NewFileStore returns a FileStore that reads/writes tokens under dir.
func NewFileStore(dir, profile string) *FileStore {
	return &FileStore{dir: dir, profile: profile}
}

// path returns the full path to the token file for this profile.
func (s *FileStore) path() string {
	return filepath.Join(s.dir, s.profile+".json")
}

// Load reads and unmarshals a token from disk. It returns nil on any error
// (file not found, corrupt JSON, etc.).
func (s *FileStore) Load() *Token {
	data, err := os.ReadFile(s.path())
	if err != nil {
		return nil
	}
	var t Token
	if err := json.Unmarshal(data, &t); err != nil {
		return nil
	}
	return &t
}

// Save writes a token to disk using atomic write (temp file + rename).
// The directory is created with 0700 permissions if it does not exist.
// The file is written with 0600 permissions.
func (s *FileStore) Save(t *Token) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}

	data, err := json.Marshal(t)
	if err != nil {
		return err
	}

	// Atomic write: write to temp file, then rename.
	tmp := s.path() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path())
}

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// goos is the operating system identifier used by DefaultPath.
// It defaults to runtime.GOOS and can be overridden in tests.
var goos = runtime.GOOS

// AuthConfig holds authentication credentials for a profile.
type AuthConfig struct {
	Type         string `json:"type"`
	Username     string `json:"username,omitempty"`
	Token        string `json:"token,omitempty"`
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	Scopes       string `json:"scopes,omitempty"`
	CloudID      string `json:"cloud_id,omitempty"`
}

// Profile holds the configuration for a named Confluence instance.
type Profile struct {
	BaseURL           string     `json:"base_url"`
	Auth              AuthConfig `json:"auth"`
	AllowedOperations []string   `json:"allowed_operations,omitempty"`
	DeniedOperations  []string   `json:"denied_operations,omitempty"`
	AuditLog          string            `json:"audit_log,omitempty"` // path to NDJSON file; empty = disabled
	Presets           map[string]string `json:"presets,omitempty"`
}

// Config is the top-level configuration structure persisted to disk.
type Config struct {
	Profiles       map[string]Profile `json:"profiles"`
	DefaultProfile string             `json:"default_profile"`
}

// FlagOverrides carries values supplied via CLI flags. Empty string means
// "not set by flag".
type FlagOverrides struct {
	BaseURL      string
	AuthType     string
	Username     string
	Token        string
	ClientID     string
	ClientSecret string
	CloudID      string
}

// ResolvedConfig is the final, merged configuration ready for use.
type ResolvedConfig struct {
	BaseURL     string
	Auth        AuthConfig
	ProfileName string
}

// DefaultPath returns the path to the configuration file. It checks the
// CF_CONFIG_PATH environment variable first; otherwise it falls back to an
// OS-specific default location.
func DefaultPath() string {
	if v := os.Getenv("CF_CONFIG_PATH"); v != "" {
		return v
	}
	switch goos {
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "cf", "config.json")
	case "windows":
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			home, _ := os.UserHomeDir()
			return filepath.Join(home, "AppData", "Roaming", "cf", "config.json")
		}
		return filepath.Join(appdata, "cf", "config.json")
	default: // linux and others
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "cf", "config.json")
	}
}

// LoadFrom reads and parses the config file at path. If the file does not
// exist, an empty (non-nil) Config is returned without error.
func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{Profiles: map[string]Profile{}}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	return &cfg, nil
}

// SaveTo serialises cfg as indented JSON and writes it to path with 0o600
// permissions, creating any missing parent directories.
// Config contains only strings and maps, so json.MarshalIndent cannot fail.
func SaveTo(cfg *Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(path, data, 0o600)
}

// availableProfiles returns a comma-separated list of profile names.
func availableProfiles(cfg *Config) string {
	names := make([]string, 0, len(cfg.Profiles))
	for k := range cfg.Profiles {
		names = append(names, k)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "(none)"
	}
	return strings.Join(names, ", ")
}

// validAuthTypes is the set of accepted authentication types.
var validAuthTypes = map[string]bool{
	"basic": true, "bearer": true,
	"oauth2": true, "oauth2-3lo": true,
}

// ValidAuthType reports whether s is a recognized authentication type (case-insensitive).
func ValidAuthType(s string) bool {
	return validAuthTypes[strings.ToLower(s)]
}

// Resolve builds a ResolvedConfig by merging sources in priority order:
// CLI flags > environment variables > config file profile > CF_PROFILE env var.
//
// The profileName argument selects which profile to load from the config
// file; an empty string falls back to CF_PROFILE env var, then
// the DefaultProfile, then "default".
func Resolve(configPath, profileName string, flags *FlagOverrides) (*ResolvedConfig, error) {
	// 1. Load from config file (lowest priority).
	cfg, err := LoadFrom(configPath)
	if err != nil {
		return nil, err
	}

	name := profileName
	// CF_PROFILE env var overrides config default_profile but is overridden by --profile flag.
	if name == "" {
		if envProfile := os.Getenv("CF_PROFILE"); envProfile != "" {
			name = envProfile
		}
	}
	if name == "" {
		name = cfg.DefaultProfile
	}
	if name == "" {
		name = "default"
	}

	var fileBaseURL, fileAuthType, fileUsername, fileToken string
	var fileClientID, fileClientSecret, fileScopes, fileCloudID string
	if p, ok := cfg.Profiles[name]; ok {
		fileBaseURL = p.BaseURL
		fileAuthType = p.Auth.Type
		fileUsername = p.Auth.Username
		fileToken = p.Auth.Token
		fileClientID = p.Auth.ClientID
		fileClientSecret = p.Auth.ClientSecret
		fileScopes = p.Auth.Scopes
		fileCloudID = p.Auth.CloudID
	} else if profileName != "" {
		// Explicit --profile that doesn't exist should give a clear error.
		return nil, fmt.Errorf("profile %q not found; available profiles: %s", name, availableProfiles(cfg))
	}

	// 2. Environment variables (override config file).
	envBaseURL := os.Getenv("CF_BASE_URL")
	envAuthType := os.Getenv("CF_AUTH_TYPE")
	envUsername := os.Getenv("CF_AUTH_USER")
	envToken := os.Getenv("CF_AUTH_TOKEN")
	envClientID := os.Getenv("CF_AUTH_CLIENT_ID")
	envClientSecret := os.Getenv("CF_AUTH_CLIENT_SECRET")
	envCloudID := os.Getenv("CF_AUTH_CLOUD_ID")

	// 3. Merge: start with file values, then layer env vars.
	baseURL := fileBaseURL
	if envBaseURL != "" {
		baseURL = envBaseURL
	}

	authType := fileAuthType
	if envAuthType != "" {
		authType = envAuthType
	}

	username := fileUsername
	if envUsername != "" {
		username = envUsername
	}

	token := fileToken
	if envToken != "" {
		token = envToken
	}

	clientID := fileClientID
	if envClientID != "" {
		clientID = envClientID
	}

	clientSecret := fileClientSecret
	if envClientSecret != "" {
		clientSecret = envClientSecret
	}

	scopes := fileScopes

	cloudID := fileCloudID
	if envCloudID != "" {
		cloudID = envCloudID
	}

	// 4. CLI flags (highest priority).
	if flags != nil {
		if flags.BaseURL != "" {
			baseURL = flags.BaseURL
		}
		if flags.AuthType != "" {
			authType = flags.AuthType
		}
		if flags.Username != "" {
			username = flags.Username
		}
		if flags.Token != "" {
			token = flags.Token
		}
		if flags.ClientID != "" {
			clientID = flags.ClientID
		}
		if flags.ClientSecret != "" {
			clientSecret = flags.ClientSecret
		}
		if flags.CloudID != "" {
			cloudID = flags.CloudID
		}
	}

	// 5. Apply defaults.
	if authType == "" {
		authType = "basic"
	}

	// 6. Validate auth type.
	if !ValidAuthType(authType) {
		return nil, fmt.Errorf("invalid auth type %q; must be one of: basic, bearer, oauth2, oauth2-3lo", authType)
	}
	authType = strings.ToLower(authType)

	// 6b. Validate OAuth2 required fields.
	if authType == "oauth2" || authType == "oauth2-3lo" {
		if clientID == "" {
			return nil, fmt.Errorf("auth type %q requires client_id; set via config, CF_AUTH_CLIENT_ID, or --client-id flag", authType)
		}
		if clientSecret == "" {
			return nil, fmt.Errorf("auth type %q requires client_secret; set via config, CF_AUTH_CLIENT_SECRET, or --client-secret flag", authType)
		}
		if cloudID == "" && authType == "oauth2" {
			return nil, fmt.Errorf("auth type %q requires cloud_id; set via config, CF_AUTH_CLOUD_ID, or --cloud-id flag", authType)
		}
	}

	// 7. Trim trailing slash from BaseURL.
	baseURL = strings.TrimRight(baseURL, "/")

	return &ResolvedConfig{
		BaseURL: baseURL,
		Auth: AuthConfig{
			Type:         authType,
			Username:     username,
			Token:        token,
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scopes:       scopes,
			CloudID:      cloudID,
		},
		ProfileName: name,
	}, nil
}

// TokenDir returns the OS-appropriate directory for storing OAuth2 token files.
func TokenDir() string {
	if v := os.Getenv("CF_TOKEN_DIR"); v != "" {
		return v
	}
	switch goos {
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "cf", "tokens")
	case "windows":
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			home, _ := os.UserHomeDir()
			return filepath.Join(home, "AppData", "Roaming", "cf", "tokens")
		}
		return filepath.Join(appdata, "cf", "tokens")
	default:
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "cf", "tokens")
	}
}

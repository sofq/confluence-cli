package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/sofq/confluence-cli/internal/config"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
	"github.com/sofq/confluence-cli/internal/jsonutil"
	"github.com/spf13/cobra"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Save connection settings to the config file (flag-driven, no prompts)",
	RunE:  runConfigure,
}

func init() {
	f := configureCmd.Flags()
	f.String("base-url", "", "Confluence base URL (required unless --delete)")
	f.String("token", "", "API token or bearer token (required unless --delete)")
	f.StringP("profile", "p", "default", "profile name to save settings under")
	f.String("auth-type", "basic", "auth type: basic, bearer, oauth2, oauth2-3lo")
	f.String("username", "", "username for basic auth")
	f.String("client-id", "", "OAuth2 client ID")
	f.String("client-secret", "", "OAuth2 client secret")
	f.String("cloud-id", "", "Atlassian Cloud site ID for OAuth2")
	f.String("scopes", "", "OAuth2 scopes (space-separated)")
	f.Bool("test", false, "test connection via GET /wiki/api/v2/spaces?limit=1 before saving")
	f.Bool("delete", false, "delete the named profile")
}

func runConfigure(cmd *cobra.Command, args []string) error {
	baseURL, _ := cmd.Flags().GetString("base-url")
	token, _ := cmd.Flags().GetString("token")
	profileName, _ := cmd.Flags().GetString("profile")
	authType, _ := cmd.Flags().GetString("auth-type")
	username, _ := cmd.Flags().GetString("username")
	clientID, _ := cmd.Flags().GetString("client-id")
	clientSecret, _ := cmd.Flags().GetString("client-secret")
	cloudID, _ := cmd.Flags().GetString("cloud-id")
	scopes, _ := cmd.Flags().GetString("scopes")
	testConn, _ := cmd.Flags().GetBool("test")
	deleteProfile, _ := cmd.Flags().GetBool("delete")

	if deleteProfile {
		// Require explicit --profile when deleting to prevent accidental
		// deletion of the default profile.
		if !cmd.Flags().Changed("profile") {
			apiErr := &cferrors.APIError{
				ErrorType: "validation_error",
				Message:   "--profile is required when using --delete (to prevent accidental deletion of the default profile)",
			}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
		return deleteProfileByName(cmd, profileName)
	}

	// Validate profile name is not empty/whitespace.
	if strings.TrimSpace(profileName) == "" {
		apiErr := &cferrors.APIError{
			ErrorType: "validation_error",
			Message:   "--profile must not be empty or whitespace-only",
		}
		apiErr.WriteJSON(os.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	// Test-only mode: when --test is set but no --base-url/--token are provided,
	// load the existing profile and test its saved credentials.
	testOnly := testConn && !cmd.Flags().Changed("base-url") && !cmd.Flags().Changed("token")
	if testOnly {
		return testExistingProfile(cmd, profileName, cmd.Flags().Changed("profile"))
	}

	// Validate auth-type before saving or testing.
	if !config.ValidAuthType(authType) {
		apiErr := &cferrors.APIError{
			ErrorType: "validation_error",
			Message:   fmt.Sprintf("invalid --auth-type %q; must be one of: basic, bearer, oauth2, oauth2-3lo", authType),
		}
		apiErr.WriteJSON(os.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}
	authType = strings.ToLower(authType)

	isOAuth2 := authType == "oauth2" || authType == "oauth2-3lo"

	// Validate required fields are not empty/whitespace.
	if strings.TrimSpace(baseURL) == "" {
		apiErr := &cferrors.APIError{
			ErrorType: "validation_error",
			Message:   "--base-url must not be empty",
		}
		apiErr.WriteJSON(os.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}
	// --token is required for basic/bearer, not for oauth2.
	if !isOAuth2 && strings.TrimSpace(token) == "" {
		apiErr := &cferrors.APIError{
			ErrorType: "validation_error",
			Message:   "--token must not be empty",
		}
		apiErr.WriteJSON(os.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	// OAuth2-specific validation.
	if isOAuth2 {
		if strings.TrimSpace(clientID) == "" {
			apiErr := &cferrors.APIError{
				ErrorType: "validation_error",
				Message:   "--client-id is required for auth-type " + authType,
			}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
		if strings.TrimSpace(clientSecret) == "" {
			apiErr := &cferrors.APIError{
				ErrorType: "validation_error",
				Message:   "--client-secret is required for auth-type " + authType,
			}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
		if authType == "oauth2" && strings.TrimSpace(cloudID) == "" {
			apiErr := &cferrors.APIError{
				ErrorType: "validation_error",
				Message:   "--cloud-id is required for auth-type oauth2",
			}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
	}

	// Normalize base URL: strip trailing slashes to avoid double-slash issues.
	baseURL = strings.TrimRight(baseURL, "/")

	if testConn {
		if err := testConnection(baseURL, authType, username, token); err != nil {
			apiErr := &cferrors.APIError{
				ErrorType: "connection_error",
				Status:    0,
				Message:   "connection test failed: " + err.Error(),
			}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
		}
	}

	configPath := config.DefaultPath()
	cfg, err := config.LoadFrom(configPath)
	if err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "config_error",
			Status:    0,
			Message:   "failed to load config: " + err.Error(),
		}
		apiErr.WriteJSON(os.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
	}

	cfg.Profiles[profileName] = config.Profile{
		BaseURL: baseURL,
		Auth: config.AuthConfig{
			Type:         authType,
			Username:     username,
			Token:        token,
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scopes:       scopes,
			CloudID:      cloudID,
		},
	}

	if cfg.DefaultProfile == "" {
		cfg.DefaultProfile = profileName
	}

	if err := config.SaveTo(cfg, configPath); err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "config_error",
			Status:    0,
			Message:   "failed to save config: " + err.Error(),
		}
		apiErr.WriteJSON(os.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
	}

	out, _ := jsonutil.MarshalNoEscape(map[string]string{
		"status":  "saved",
		"profile": profileName,
		"path":    configPath,
	})
	return schemaOutput(cmd, out)
}

// testExistingProfile loads a saved profile and tests its connection.
// profileExplicit indicates whether --profile was explicitly passed by the user.
func testExistingProfile(cmd *cobra.Command, profileName string, profileExplicit bool) error {
	configPath := config.DefaultPath()
	cfg, err := config.LoadFrom(configPath)
	if err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "config_error",
			Status:    0,
			Message:   "failed to load config: " + err.Error(),
		}
		apiErr.WriteJSON(os.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
	}

	// Resolve profile name: use default_profile only if --profile was not
	// explicitly set (i.e. we're using the flag's default value of "default").
	name := profileName
	if !profileExplicit && name == "default" {
		if cfg.DefaultProfile != "" {
			name = cfg.DefaultProfile
		}
	}

	profile, ok := cfg.Profiles[name]
	if !ok {
		availableNames := make([]string, 0, len(cfg.Profiles))
		for k := range cfg.Profiles {
			availableNames = append(availableNames, k)
		}
		sort.Strings(availableNames)
		apiErr := &cferrors.APIError{
			ErrorType: "not_found",
			Message:   fmt.Sprintf("profile %q not found; available profiles: %s", name, strings.Join(availableNames, ", ")),
		}
		apiErr.WriteJSON(os.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitNotFound}
	}

	if strings.TrimSpace(profile.BaseURL) == "" {
		apiErr := &cferrors.APIError{
			ErrorType: "validation_error",
			Message:   fmt.Sprintf("profile %q has no base_url configured", name),
		}
		apiErr.WriteJSON(os.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	if err := testConnection(profile.BaseURL, profile.Auth.Type, profile.Auth.Username, profile.Auth.Token); err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "connection_error",
			Status:    0,
			Message:   "connection test failed: " + err.Error(),
		}
		apiErr.WriteJSON(os.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
	}

	out, _ := jsonutil.MarshalNoEscape(map[string]string{
		"status":  "ok",
		"profile": name,
	})
	return schemaOutput(cmd, out)
}

// deleteProfileByName removes a profile from the config file.
func deleteProfileByName(cmd *cobra.Command, name string) error {
	configPath := config.DefaultPath()
	cfg, err := config.LoadFrom(configPath)
	if err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "config_error",
			Message:   "failed to load config: " + err.Error(),
		}
		apiErr.WriteJSON(os.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
	}

	if _, ok := cfg.Profiles[name]; !ok {
		availableNames := make([]string, 0, len(cfg.Profiles))
		for k := range cfg.Profiles {
			availableNames = append(availableNames, k)
		}
		sort.Strings(availableNames)
		apiErr := &cferrors.APIError{
			ErrorType: "not_found",
			Message:   fmt.Sprintf("profile %q not found; available profiles: %s", name, strings.Join(availableNames, ", ")),
		}
		apiErr.WriteJSON(os.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitNotFound}
	}

	delete(cfg.Profiles, name)
	if cfg.DefaultProfile == name {
		cfg.DefaultProfile = ""
	}

	if err := config.SaveTo(cfg, configPath); err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "config_error",
			Message:   "failed to save config: " + err.Error(),
		}
		apiErr.WriteJSON(os.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
	}

	out, _ := jsonutil.MarshalNoEscape(map[string]string{
		"status":  "deleted",
		"profile": name,
		"path":    configPath,
	})
	return schemaOutput(cmd, out)
}

// testConnection performs a GET /wiki/api/v2/spaces?limit=1 against baseURL to verify credentials.
func testConnection(baseURL, authType, username, token string) error {
	baseURL = strings.TrimRight(baseURL, "/")
	prefix := "/wiki/api/v2"
	if strings.HasSuffix(baseURL, prefix) {
		prefix = ""
	}
	testURL := baseURL + prefix + "/spaces?limit=1"
	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	switch strings.ToLower(authType) {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+token)
	default: // basic
		req.SetBasicAuth(username, token)
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

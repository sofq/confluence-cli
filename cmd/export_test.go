// export_test.go exposes internal cmd package symbols for white-box testing.
// This file is compiled only during tests (package cmd_test import).
package cmd

import (
	"context"
	"io"

	"github.com/sofq/confluence-cli/cmd/generated"
	"github.com/sofq/confluence-cli/internal/client"
	cftemplate "github.com/sofq/confluence-cli/internal/template"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// ResolveSpaceID exposes the package-private resolveSpaceID helper for tests.
func ResolveSpaceID(ctx context.Context, c *client.Client, keyOrID string) (string, int) {
	return resolveSpaceID(ctx, c, keyOrID)
}

// FetchPageVersion exposes the package-private fetchPageVersion helper for tests.
func FetchPageVersion(ctx context.Context, c *client.Client, id string) (int, int) {
	return fetchPageVersion(ctx, c, id)
}

// DoPageUpdate exposes the package-private doPageUpdate helper for tests.
func DoPageUpdate(ctx context.Context, c *client.Client, id, title, storageValue string, versionNumber int) int {
	return doPageUpdate(ctx, c, id, title, storageValue, versionNumber)
}

// SearchV1Domain exposes client.SearchV1Domain for tests.
func SearchV1Domain(baseURL string) string {
	return client.SearchV1Domain(baseURL)
}

// ExecuteBatchOps exposes executeBatchOp for direct testing without the full CLI
// entrypoint. It builds the opMap from generated.AllSchemaOps() and executes each
// op using a background context and the provided client.
func ExecuteBatchOps(c *client.Client, ops []BatchOp) []BatchResult {
	allOps := generated.AllSchemaOps()
	opMap := make(map[string]generated.SchemaOp, len(allOps))
	for _, op := range allOps {
		key := op.Resource + " " + op.Verb
		opMap[key] = op
	}
	ctx := context.Background()
	results := make([]BatchResult, len(ops))
	for i, bop := range ops {
		results[i] = executeBatchOp(ctx, c, i, bop, opMap)
	}
	return results
}

// FetchBlogpostVersion exposes the package-private fetchBlogpostVersion helper for tests.
func FetchBlogpostVersion(ctx context.Context, c *client.Client, id string) (int, int) {
	return fetchBlogpostVersion(ctx, c, id)
}

// DoBlogpostUpdate exposes the package-private doBlogpostUpdate helper for tests.
func DoBlogpostUpdate(ctx context.Context, c *client.Client, id, title, storageValue string, versionNumber int) int {
	return doBlogpostUpdate(ctx, c, id, title, storageValue, versionNumber)
}

// FetchCustomContentMeta exposes the package-private fetchCustomContentMeta helper for tests.
func FetchCustomContentMeta(ctx context.Context, c *client.Client, id string) (customContentMeta, int) {
	return fetchCustomContentMeta(ctx, c, id)
}

// DoCustomContentUpdate exposes the package-private doCustomContentUpdate helper for tests.
func DoCustomContentUpdate(ctx context.Context, c *client.Client, id, ccType, title, storageValue string, versionNumber int) int {
	return doCustomContentUpdate(ctx, c, id, ccType, title, storageValue, versionNumber)
}

// FetchV1 exposes the package-private fetchV1 helper for tests.
func FetchV1(cmd *cobra.Command, c *client.Client, fullURL string) ([]byte, int) {
	return fetchV1(cmd, c, fullURL)
}

// FetchV1WithBody exposes the package-private fetchV1WithBody helper for tests.
func FetchV1WithBody(cmd *cobra.Command, c *client.Client, method, fullURL string, body io.Reader) ([]byte, int) {
	return fetchV1WithBody(cmd, c, method, fullURL, body)
}

// ResolveTemplate exposes the package-private resolveTemplate helper for tests.
func ResolveTemplate(w io.Writer, templateName string, varFlags []string) (*cftemplate.RenderedTemplate, error) {
	return resolveTemplate(w, templateName, varFlags)
}

// RunSearch exposes runSearch for tests.
func RunSearch(cmd *cobra.Command, args []string) error {
	return runSearch(cmd, args)
}

// resetPFlag resets a pflag to its default value and marks it not-Changed.
func resetPFlag(fs *pflag.FlagSet, name, defVal string) {
	if f := fs.Lookup(name); f != nil {
		f.Changed = false
		_ = f.Value.Set(defVal)
	}
}

// ResetConfigureFlags resets cobra flag Changed state on configureCmd's local flags.
// This is necessary because cobra reuses flag values between Execute() calls when using
// the singleton rootCmd. Without this, flags set in one test bleed into subsequent tests
// that rely on Changed=false (e.g., to detect test-only mode).
func ResetConfigureFlags() {
	fs := configureCmd.Flags()
	resetPFlag(fs, "base-url", "")
	resetPFlag(fs, "token", "")
	resetPFlag(fs, "test", "false")
	resetPFlag(fs, "delete", "false")
	resetPFlag(fs, "profile", "default")
	resetPFlag(fs, "auth-type", "basic")
	resetPFlag(fs, "username", "")
	resetPFlag(fs, "client-id", "")
	resetPFlag(fs, "client-secret", "")
	resetPFlag(fs, "cloud-id", "")
	resetPFlag(fs, "scopes", "")
}

// ResetRootPersistentFlags resets cobra persistent flag Changed state on rootCmd,
// and also resets local flags on well-known subcommands that can bleed state.
// Persistent flags like --jq, --pretty etc. can bleed between tests when using
// the singleton rootCmd pattern.
func ResetRootPersistentFlags() {
	fs := rootCmd.PersistentFlags()
	resetPFlag(fs, "jq", "")
	resetPFlag(fs, "preset", "")
	resetPFlag(fs, "pretty", "false")
	resetPFlag(fs, "no-paginate", "false")
	resetPFlag(fs, "verbose", "false")
	resetPFlag(fs, "dry-run", "false")
	resetPFlag(fs, "fields", "")
	resetPFlag(fs, "profile", "")
	resetPFlag(fs, "base-url", "")
	resetPFlag(fs, "auth-type", "")
	resetPFlag(fs, "auth-user", "")
	resetPFlag(fs, "auth-token", "")
	resetPFlag(fs, "audit", "")
	resetPFlag(fs, "cache", "")
	resetPFlag(fs, "timeout", "30s")

	// Reset schemaCmd local flags.
	sfs := schemaCmd.Flags()
	resetPFlag(sfs, "list", "false")
	resetPFlag(sfs, "compact", "false")

	// Reset rawCmd local flags.
	rfs := rawCmd.Flags()
	resetPFlag(rfs, "body", "")
	// --query is a StringArray flag. Mark Changed=false and clear via Replace if supported.
	if f := rfs.Lookup("query"); f != nil {
		f.Changed = false
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			_ = sv.Replace(nil)
		}
	}

	// Reset batchCmd local flags.
	bfs := batchCmd.Flags()
	resetPFlag(bfs, "input", "")
	resetPFlag(bfs, "max-batch", "50")

	// Reset labels subcommand local flags.
	// labels_add has a StringSlice --label flag that accumulates between test runs.
	if f := labels_add.Flags().Lookup("label"); f != nil {
		f.Changed = false
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			_ = sv.Replace(nil)
		}
	}
	resetPFlag(labels_add.Flags(), "page-id", "")
	resetPFlag(labels_remove.Flags(), "page-id", "")
	resetPFlag(labels_remove.Flags(), "label", "")
	resetPFlag(labels_list.Flags(), "page-id", "")
}

// ParseErrorJSON exposes the package-private parseErrorJSON helper for tests.
func ParseErrorJSON(errOutput string) []byte {
	return []byte(parseErrorJSON(errOutput))
}

// StripVerboseLogs exposes the package-private stripVerboseLogs helper for tests.
func StripVerboseLogs(stderrStr string) string {
	return stripVerboseLogs(stderrStr)
}

// SchemaOutput exposes the package-private schemaOutput helper for tests.
func SchemaOutput(cmd *cobra.Command, data []byte) error {
	return schemaOutput(cmd, data)
}

// LabelsAddValidation validates the inputs for the labels add command without
// making any HTTP requests. Returns 0 (ExitOK) if valid, non-zero if invalid.
func LabelsAddValidation(pageID string, labelNames []string) int {
	if pageID == "" {
		return 4 // ExitValidation
	}
	if len(labelNames) == 0 {
		return 4 // ExitValidation
	}
	hasNonEmpty := false
	for _, n := range labelNames {
		if n != "" {
			hasNonEmpty = true
			break
		}
	}
	if !hasNonEmpty {
		return 4 // ExitValidation
	}
	return 0 // ExitOK
}

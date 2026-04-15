// export_test.go exposes internal cmd package symbols for white-box testing.
// This file is compiled only during tests (package cmd_test import).
package cmd

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/sofq/confluence-cli/cmd/generated"
	"github.com/sofq/confluence-cli/internal/client"
	"github.com/sofq/confluence-cli/internal/oauth2"
	preset_pkg "github.com/sofq/confluence-cli/internal/preset"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// SetPresetUserPresetsPath overrides the userPresetsPath function in the preset package for tests.
// Returns the previous function so callers can restore it.
func SetPresetUserPresetsPath(f func() string) func() string {
	return preset_pkg.SetUserPresetsPath(f)
}

// SetOAuth2TokenEndpoint overrides the OAuth2 client-credentials token endpoint for tests.
func SetOAuth2TokenEndpoint(url string) { oauth2.SetTokenEndpoint(url) }

// SetOAuth2TokenEndpointThreeLO overrides the OAuth2 3LO token endpoint for tests.
func SetOAuth2TokenEndpointThreeLO(url string) { oauth2.SetTokenEndpointThreeLO(url) }

// SetOAuth2CallbackTimeout overrides the OAuth2 3LO browser callback timeout for tests.
func SetOAuth2CallbackTimeout(d time.Duration) { oauth2.SetCallbackTimeout(d) }

// ResolveSpaceID exposes the package-private resolveSpaceID helper for tests.
func ResolveSpaceID(ctx context.Context, c *client.Client, keyOrID string) (string, int) {
	return resolveSpaceID(ctx, c, keyOrID)
}

// RunSpacesListCmd exposes the spaces_workflow_list RunE for direct testing.
func RunSpacesListCmd(cmd *cobra.Command, args []string) error {
	return spaces_workflow_list.RunE(cmd, args)
}

// RunSpacesGetByIDCmd exposes the spaces_workflow_get_by_id RunE for direct testing.
func RunSpacesGetByIDCmd(cmd *cobra.Command, args []string) error {
	return spaces_workflow_get_by_id.RunE(cmd, args)
}

// RunPagesWorkflowCreate exposes the pages_workflow_create RunE for direct testing.
func RunPagesWorkflowCreate(cmd *cobra.Command, args []string) error {
	return pages_workflow_create.RunE(cmd, args)
}

// RunPagesWorkflowUpdate exposes the pages_workflow_update RunE for direct testing.
func RunPagesWorkflowUpdate(cmd *cobra.Command, args []string) error {
	return pages_workflow_update.RunE(cmd, args)
}

// RunPagesWorkflowDelete exposes the pages_workflow_delete RunE for direct testing.
func RunPagesWorkflowDelete(cmd *cobra.Command, args []string) error {
	return pages_workflow_delete.RunE(cmd, args)
}

// RunPagesWorkflowGetByID exposes the pages_workflow_get_by_id RunE for direct testing.
func RunPagesWorkflowGetByID(cmd *cobra.Command, args []string) error {
	return pages_workflow_get_by_id.RunE(cmd, args)
}

// RunPagesWorkflowList exposes the pages_workflow_list RunE for direct testing.
func RunPagesWorkflowList(cmd *cobra.Command, args []string) error {
	return pages_workflow_list.RunE(cmd, args)
}

// RunCustomContentWorkflowCreate exposes the custom_content_workflow_create RunE for direct testing.
func RunCustomContentWorkflowCreate(cmd *cobra.Command, args []string) error {
	return custom_content_workflow_create.RunE(cmd, args)
}

// RunCustomContentWorkflowUpdate exposes the custom_content_workflow_update RunE for direct testing.
func RunCustomContentWorkflowUpdate(cmd *cobra.Command, args []string) error {
	return custom_content_workflow_update.RunE(cmd, args)
}

// RunCustomContentWorkflowDelete exposes the custom_content_workflow_delete RunE for direct testing.
func RunCustomContentWorkflowDelete(cmd *cobra.Command, args []string) error {
	return custom_content_workflow_delete.RunE(cmd, args)
}

// RunCustomContentWorkflowGetByID exposes the custom_content_workflow_get_by_id RunE for direct testing.
func RunCustomContentWorkflowGetByID(cmd *cobra.Command, args []string) error {
	return custom_content_workflow_get_by_id.RunE(cmd, args)
}

// RunCustomContentWorkflowGetByType exposes the custom_content_workflow_get_by_type RunE for direct testing.
func RunCustomContentWorkflowGetByType(cmd *cobra.Command, args []string) error {
	return custom_content_workflow_get_by_type.RunE(cmd, args)
}

// SpacesWorkflowListCmd exposes the spaces_workflow_list command reference so
// tests can set flags directly.
func SpacesWorkflowListCmd() *cobra.Command { return spaces_workflow_list }

// SpacesWorkflowGetByIDCmd exposes the spaces_workflow_get_by_id command reference.
func SpacesWorkflowGetByIDCmd() *cobra.Command { return spaces_workflow_get_by_id }

// PagesCmd exposes the pagesCmd parent command for coverage of its RunE.
func PagesCmd() *cobra.Command { return pagesCmd }

// SpacesCmd exposes the spacesCmd parent command for coverage of its RunE.
func SpacesCmd() *cobra.Command { return spacesCmd }

// CustomContentCmd exposes the custom_contentCmd parent command for coverage of its RunE.
func CustomContentCmd() *cobra.Command { return custom_contentCmd }

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
	allOps = append(allOps, DiffSchemaOps()...)
	allOps = append(allOps, WorkflowSchemaOps()...)
	allOps = append(allOps, ExportSchemaOps()...)
	allOps = append(allOps, PresetSchemaOps()...)
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

	// Reset pages workflow subcommand local flags.
	resetPFlag(pages_workflow_get_by_id.Flags(), "id", "")
	resetPFlag(pages_workflow_get_by_id.Flags(), "body-format", "storage")
	resetPFlag(pages_workflow_create.Flags(), "space-id", "")
	resetPFlag(pages_workflow_create.Flags(), "title", "")
	resetPFlag(pages_workflow_create.Flags(), "body", "")
	resetPFlag(pages_workflow_create.Flags(), "parent-id", "")
	resetPFlag(pages_workflow_create.Flags(), "template", "")
	if f := pages_workflow_create.Flags().Lookup("var"); f != nil {
		f.Changed = false
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			_ = sv.Replace(nil)
		}
	}
	resetPFlag(pages_workflow_update.Flags(), "id", "")
	resetPFlag(pages_workflow_update.Flags(), "title", "")
	resetPFlag(pages_workflow_update.Flags(), "body", "")
	resetPFlag(pages_workflow_delete.Flags(), "id", "")
	resetPFlag(pages_workflow_list.Flags(), "space-id", "")

	// Reset spaces workflow subcommand local flags.
	resetPFlag(spaces_workflow_list.Flags(), "key", "")
	resetPFlag(spaces_workflow_get_by_id.Flags(), "id", "")

	// Reset custom_content workflow subcommand local flags.
	resetPFlag(custom_content_workflow_get_by_type.Flags(), "type", "")
	resetPFlag(custom_content_workflow_get_by_type.Flags(), "space-id", "")
	resetPFlag(custom_content_workflow_create.Flags(), "type", "")
	resetPFlag(custom_content_workflow_create.Flags(), "space-id", "")
	resetPFlag(custom_content_workflow_create.Flags(), "title", "")
	resetPFlag(custom_content_workflow_create.Flags(), "body", "")
	resetPFlag(custom_content_workflow_get_by_id.Flags(), "id", "")
	resetPFlag(custom_content_workflow_get_by_id.Flags(), "body-format", "storage")
	resetPFlag(custom_content_workflow_update.Flags(), "id", "")
	resetPFlag(custom_content_workflow_update.Flags(), "type", "")
	resetPFlag(custom_content_workflow_update.Flags(), "title", "")
	resetPFlag(custom_content_workflow_update.Flags(), "body", "")
	resetPFlag(custom_content_workflow_delete.Flags(), "id", "")

	// Reset attachments subcommand local flags.
	resetPFlag(attachments_workflow_list.Flags(), "page-id", "")
	resetPFlag(attachments_workflow_upload.Flags(), "page-id", "")
	resetPFlag(attachments_workflow_upload.Flags(), "file", "")

	// Reset blogposts subcommand local flags.
	resetPFlag(blogposts_workflow_get_by_id.Flags(), "id", "")
	resetPFlag(blogposts_workflow_get_by_id.Flags(), "body-format", "storage")
	resetPFlag(blogposts_workflow_create.Flags(), "space-id", "")
	resetPFlag(blogposts_workflow_create.Flags(), "title", "")
	resetPFlag(blogposts_workflow_create.Flags(), "body", "")
	resetPFlag(blogposts_workflow_create.Flags(), "template", "")
	if f := blogposts_workflow_create.Flags().Lookup("var"); f != nil {
		f.Changed = false
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			_ = sv.Replace(nil)
		}
	}
	resetPFlag(blogposts_workflow_update.Flags(), "id", "")
	resetPFlag(blogposts_workflow_update.Flags(), "title", "")
	resetPFlag(blogposts_workflow_update.Flags(), "body", "")
	resetPFlag(blogposts_workflow_delete.Flags(), "id", "")
	resetPFlag(blogposts_workflow_list.Flags(), "space-id", "")

	// Reset comments subcommand local flags.
	resetPFlag(comments_list.Flags(), "page-id", "")
	resetPFlag(comments_create.Flags(), "page-id", "")
	resetPFlag(comments_create.Flags(), "body", "")
	resetPFlag(comments_delete.Flags(), "comment-id", "")

	// Reset rootCmd output/error writers so they fall back to os.Stdout/os.Stderr.
	// Tests that call root.SetOut(buf) leave the singleton with a stale writer which
	// causes subsequent tests that redirect os.Stdout (e.g. TestVersionFlagOutputsJSON)
	// to see empty output because schemaOutput / cobra version template write via the
	// command writer, not directly to os.Stdout.
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
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

// PollLongTask exposes the package-private pollLongTask helper for tests.
// This allows tests to call it directly with arbitrary time.Duration values
// rather than being limited to the custom duration.Parse format used by the CLI.
func PollLongTask(ctx context.Context, cmd *cobra.Command, c *client.Client, taskID string, timeout time.Duration) ([]byte, int) {
	return pollLongTask(ctx, cmd, c, taskID, timeout)
}

// RunBatch exposes the package-private runBatch RunE function for direct testing.
func RunBatch(cmd *cobra.Command, args []string) error {
	return runBatch(cmd, args)
}

// RunDiff exposes the package-private runDiff RunE function for direct testing.
func RunDiff(cmd *cobra.Command, args []string) error {
	return runDiff(cmd, args)
}

// RunExport exposes the package-private runExport RunE function for direct testing.
func RunExport(cmd *cobra.Command, args []string) error {
	return runExport(cmd, args)
}

// RunWatch exposes the package-private runWatch RunE function for direct testing.
func RunWatch(cmd *cobra.Command, args []string) error {
	return runWatch(cmd, args)
}

// RunRaw exposes the package-private runRaw RunE function for direct testing.
func RunRaw(cmd *cobra.Command, args []string) error {
	return runRaw(cmd, args)
}

// RunWorkflowMove exposes the package-private runWorkflowMove for direct testing.
func RunWorkflowMove(cmd *cobra.Command, args []string) error {
	return runWorkflowMove(cmd, args)
}

// RunWorkflowCopy exposes the package-private runWorkflowCopy for direct testing.
func RunWorkflowCopy(cmd *cobra.Command, args []string) error {
	return runWorkflowCopy(cmd, args)
}

// RunWorkflowPublish exposes the package-private runWorkflowPublish for direct testing.
func RunWorkflowPublish(cmd *cobra.Command, args []string) error {
	return runWorkflowPublish(cmd, args)
}

// RunWorkflowComment exposes the package-private runWorkflowComment for direct testing.
func RunWorkflowComment(cmd *cobra.Command, args []string) error {
	return runWorkflowComment(cmd, args)
}

// RunWorkflowRestrict exposes the package-private runWorkflowRestrict for direct testing.
func RunWorkflowRestrict(cmd *cobra.Command, args []string) error {
	return runWorkflowRestrict(cmd, args)
}

// RunWorkflowArchive exposes the package-private runWorkflowArchive for direct testing.
func RunWorkflowArchive(cmd *cobra.Command, args []string) error {
	return runWorkflowArchive(cmd, args)
}

// RunLabelsListCmd exposes the labels_list RunE for direct testing.
func RunLabelsListCmd(cmd *cobra.Command, args []string) error {
	return labels_list.RunE(cmd, args)
}

// RunLabelsAddCmd exposes the labels_add RunE for direct testing.
func RunLabelsAddCmd(cmd *cobra.Command, args []string) error {
	return labels_add.RunE(cmd, args)
}

// RunLabelsRemoveCmd exposes the labels_remove RunE for direct testing.
func RunLabelsRemoveCmd(cmd *cobra.Command, args []string) error {
	return labels_remove.RunE(cmd, args)
}

// FetchVersionList exposes the package-private fetchVersionList helper for tests.
// This allows context-cancellation to be tested without going through the CLI.
func FetchVersionList(ctx context.Context, c *client.Client, pageID string, limit int) ([]apiVersionEntry, error) {
	return fetchVersionList(ctx, c, pageID, limit)
}

// WalkTree exposes the package-private walkTree helper for tests.
// This allows context-cancellation to be tested directly.
func WalkTree(ctx context.Context, c *client.Client, pageID, parentID string,
	currentDepth, maxDepth int, format string, enc *json.Encoder) {
	walkTree(ctx, c, pageID, parentID, currentDepth, maxDepth, format, enc)
}

// FetchAllChildren exposes the package-private fetchAllChildren helper for tests.
func FetchAllChildren(ctx context.Context, c *client.Client, pageID string) ([]childInfo, error) {
	return fetchAllChildren(ctx, c, pageID)
}

// PollAndEmit exposes the package-private pollAndEmit helper for tests.
func PollAndEmit(ctx context.Context, watchCmd *cobra.Command, c *client.Client, cqlQuery string, seen map[string]time.Time, enc *json.Encoder) error {
	return pollAndEmit(ctx, watchCmd, c, cqlQuery, seen, enc)
}

// RunWatchInLoop exposes the inner watch select loop path for testing ctx.Done().
// It starts the watch command and returns immediately; the caller must cancel the
// context to trigger the shutdown path.
// NOTE: This is intentionally unused for the loop test — see TestWatch_CtxDoneInLoop.
func RunWatchSelectCtxDone(ctx context.Context, c *client.Client) {
	enc := json.NewEncoder(c.Stdout)
	seen := make(map[string]time.Time)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = enc.Encode(map[string]string{"type": "shutdown"})
			return
		case <-ticker.C:
			_ = pollAndEmit(ctx, nil, c, "type=page", seen, enc)
			return
		}
	}
}

// RunAttachmentsListCmd exposes attachments_workflow_list.RunE for direct testing.
func RunAttachmentsListCmd(cobraCmd *cobra.Command, args []string) error {
	return attachments_workflow_list.RunE(cobraCmd, args)
}

// RunAttachmentsUploadCmd exposes attachments_workflow_upload.RunE for direct testing.
func RunAttachmentsUploadCmd(cobraCmd *cobra.Command, args []string) error {
	return attachments_workflow_upload.RunE(cobraCmd, args)
}

// RunBlogpostsGetByIDCmd exposes blogposts_workflow_get_by_id.RunE for direct testing.
func RunBlogpostsGetByIDCmd(cobraCmd *cobra.Command, args []string) error {
	return blogposts_workflow_get_by_id.RunE(cobraCmd, args)
}

// RunBlogpostsCreateCmd exposes blogposts_workflow_create.RunE for direct testing.
func RunBlogpostsCreateCmd(cobraCmd *cobra.Command, args []string) error {
	return blogposts_workflow_create.RunE(cobraCmd, args)
}

// RunBlogpostsUpdateCmd exposes blogposts_workflow_update.RunE for direct testing.
func RunBlogpostsUpdateCmd(cobraCmd *cobra.Command, args []string) error {
	return blogposts_workflow_update.RunE(cobraCmd, args)
}

// RunBlogpostsDeleteCmd exposes blogposts_workflow_delete.RunE for direct testing.
func RunBlogpostsDeleteCmd(cobraCmd *cobra.Command, args []string) error {
	return blogposts_workflow_delete.RunE(cobraCmd, args)
}

// RunBlogpostsListCmd exposes blogposts_workflow_list.RunE for direct testing.
func RunBlogpostsListCmd(cobraCmd *cobra.Command, args []string) error {
	return blogposts_workflow_list.RunE(cobraCmd, args)
}

// RunCommentsListCmd exposes comments_list.RunE for direct testing.
func RunCommentsListCmd(cobraCmd *cobra.Command, args []string) error {
	return comments_list.RunE(cobraCmd, args)
}

// RunCommentsCreateCmd exposes comments_create.RunE for direct testing.
func RunCommentsCreateCmd(cobraCmd *cobra.Command, args []string) error {
	return comments_create.RunE(cobraCmd, args)
}

// RunCommentsDeleteCmd exposes comments_delete.RunE for direct testing.
func RunCommentsDeleteCmd(cobraCmd *cobra.Command, args []string) error {
	return comments_delete.RunE(cobraCmd, args)
}

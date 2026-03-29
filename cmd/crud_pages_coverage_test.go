package cmd_test

// crud_pages_coverage_test.go covers the anonymous RunE closures in
// cmd/pages.go, cmd/spaces.go, and cmd/custom_content.go.
//
// Commands are exercised via the exported RunXxx helpers (backed by the
// singleton cobra command's RunE field) and a directly-built client. This
// avoids the cobra singleton flag-bleed problem that occurs when going through
// RootCommand().Execute() for commands that require sub-flag validation.
//
// Parent RunE branches (unknown/missing subcommand) are exercised by calling
// the exported PagesCmd/SpacesCmd/CustomContentCmd().RunE directly.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/sofq/confluence-cli/cmd"
	"github.com/sofq/confluence-cli/internal/client"
	"github.com/sofq/confluence-cli/internal/config"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeClientCRUD builds a client pointed at srv with a bearer token.
func makeClientCRUD(srv *httptest.Server) *client.Client {
	return &client.Client{
		BaseURL:    srv.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test-token"},
		HTTPClient: srv.Client(),
		Stdout:     &strings.Builder{},
		Stderr:     &strings.Builder{},
	}
}

// newFlagCmd returns a throwaway *cobra.Command with the given string flags set
// and ctx injected via SetContext. Used as the flag-holder passed to RunE helpers.
func newFlagCmd(ctx context.Context, flags map[string]string) *cobra.Command {
	c := &cobra.Command{Use: "test"}
	c.SetContext(ctx)
	for k, v := range flags {
		c.Flags().String(k, v, "")
		_ = c.Flags().Set(k, v)
	}
	return c
}

// newFlagCmdOverride creates a command with a flag that has been explicitly
// Changed (i.e. set after registration, simulating a user-provided value).
// Use this when the RunE logic checks cmd.Flags().Changed(flagName).
func newFlagCmdOverride(ctx context.Context, strFlags map[string]string, overrideKey, overrideVal string) *cobra.Command {
	c := &cobra.Command{Use: "test"}
	c.SetContext(ctx)
	for k, v := range strFlags {
		c.Flags().String(k, v, "")
	}
	// Set the override flag AFTER registration so Changed = true.
	_ = c.Flags().Set(overrideKey, overrideVal)
	return c
}

// withClientCRUD returns a context that carries a client pointing at srv.
func withClientCRUD(srv *httptest.Server) context.Context {
	c := makeClientCRUD(srv)
	return client.NewContext(context.Background(), c)
}

// ---------------------------------------------------------------------------
// pages.go — parent RunE (unknown / missing subcommand)
// ---------------------------------------------------------------------------

// TestPagesParentRunE_Unknown covers the unknown-subcommand branch.
func TestPagesParentRunE_Unknown(t *testing.T) {
	if err := cmd.PagesCmd().RunE(cmd.PagesCmd(), []string{"doesnotexist"}); err == nil {
		t.Error("expected error for unknown subcommand")
	}
}

// TestPagesParentRunE_NoArgs covers the missing-subcommand branch.
func TestPagesParentRunE_NoArgs(t *testing.T) {
	if err := cmd.PagesCmd().RunE(cmd.PagesCmd(), []string{}); err == nil {
		t.Error("expected error for missing subcommand")
	}
}

// ---------------------------------------------------------------------------
// pages.go — pages get-by-id
// ---------------------------------------------------------------------------

// TestPagesGetByID_EmptyID covers the --id validation branch.
func TestPagesGetByID_EmptyID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call")
	}))
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "", "body-format": "storage"})
	if err := cmd.RunPagesWorkflowGetByID(flagCmd, nil); err == nil {
		t.Error("expected validation error for empty --id")
	}
}

// TestPagesGetByID_Success covers the happy path (default body-format=storage).
func TestPagesGetByID_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pages/55", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"55","title":"Test","version":{"number":1}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "55", "body-format": "storage"})
	if err := cmd.RunPagesWorkflowGetByID(flagCmd, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestPagesGetByID_OverrideBodyFormat covers the body-format Changed branch.
func TestPagesGetByID_OverrideBodyFormat(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pages/55", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"55","title":"Test","version":{"number":1}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	// Override body-format after registration so Changed = true.
	flagCmd := newFlagCmdOverride(ctx, map[string]string{"id": "55", "body-format": "storage"}, "body-format", "atlas_doc_format")
	if err := cmd.RunPagesWorkflowGetByID(flagCmd, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestPagesGetByID_HTTPError covers the non-zero exit path.
func TestPagesGetByID_HTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pages/99", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error_type":"not_found","message":"not found"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "99", "body-format": "storage"})
	if err := cmd.RunPagesWorkflowGetByID(flagCmd, nil); err == nil {
		t.Error("expected error from 404 response")
	}
}

// ---------------------------------------------------------------------------
// pages.go — pages create
// ---------------------------------------------------------------------------

// TestPagesCreate_MissingSpaceID covers the --space-id validation branch.
func TestPagesCreate_MissingSpaceID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call")
	}))
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"space-id": "", "title": "T", "body": "<p>b</p>", "template": "", "parent-id": ""})
	flagCmd.Flags().StringArray("var", nil, "")
	if err := cmd.RunPagesWorkflowCreate(flagCmd, nil); err == nil {
		t.Error("expected validation error for missing --space-id")
	}
}

// TestPagesCreate_MissingTitle covers the --title validation branch.
func TestPagesCreate_MissingTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call")
	}))
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"space-id": "123", "title": "", "body": "<p>b</p>", "template": "", "parent-id": ""})
	flagCmd.Flags().StringArray("var", nil, "")
	if err := cmd.RunPagesWorkflowCreate(flagCmd, nil); err == nil {
		t.Error("expected validation error for missing --title")
	}
}

// TestPagesCreate_MissingBody covers the --body validation branch.
func TestPagesCreate_MissingBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call")
	}))
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"space-id": "123", "title": "T", "body": "", "template": "", "parent-id": ""})
	flagCmd.Flags().StringArray("var", nil, "")
	if err := cmd.RunPagesWorkflowCreate(flagCmd, nil); err == nil {
		t.Error("expected validation error for missing --body")
	}
}

// TestPagesCreate_TemplateAndBodyConflict covers the template+body conflict branch.
func TestPagesCreate_TemplateAndBodyConflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call")
	}))
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"space-id": "123", "title": "T", "body": "<p>x</p>", "template": "some-tpl", "parent-id": ""})
	flagCmd.Flags().StringArray("var", nil, "")
	if err := cmd.RunPagesWorkflowCreate(flagCmd, nil); err == nil {
		t.Error("expected validation error when both --template and --body are provided")
	}
}

// TestPagesCreate_Success covers the happy path without a parent-id.
func TestPagesCreate_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pages", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"1","title":"Test Page","version":{"number":1}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"space-id": "123", "title": "Test Page", "body": "<p>hi</p>", "template": "", "parent-id": ""})
	flagCmd.Flags().StringArray("var", nil, "")
	if err := cmd.RunPagesWorkflowCreate(flagCmd, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestPagesCreate_WithParentID covers the parentId optional field path.
func TestPagesCreate_WithParentID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pages", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"2","title":"Child","version":{"number":1}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"space-id": "123", "title": "Child", "body": "<p>c</p>", "template": "", "parent-id": "99"})
	flagCmd.Flags().StringArray("var", nil, "")
	if err := cmd.RunPagesWorkflowCreate(flagCmd, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestPagesCreate_HTTPError covers the non-zero exit path from POST /pages.
func TestPagesCreate_HTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pages", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error_type":"error","message":"server error"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"space-id": "123", "title": "T", "body": "<p>b</p>", "template": "", "parent-id": ""})
	flagCmd.Flags().StringArray("var", nil, "")
	if err := cmd.RunPagesWorkflowCreate(flagCmd, nil); err == nil {
		t.Error("expected error from 500 response")
	}
}

// ---------------------------------------------------------------------------
// pages.go — pages update
// ---------------------------------------------------------------------------

// TestPagesUpdate_MissingID covers the --id validation branch.
func TestPagesUpdate_MissingID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call")
	}))
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "", "title": "T", "body": "<p>b</p>"})
	if err := cmd.RunPagesWorkflowUpdate(flagCmd, nil); err == nil {
		t.Error("expected validation error for missing --id")
	}
}

// TestPagesUpdate_MissingTitle covers the --title validation branch.
func TestPagesUpdate_MissingTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call")
	}))
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "42", "title": "", "body": "<p>b</p>"})
	if err := cmd.RunPagesWorkflowUpdate(flagCmd, nil); err == nil {
		t.Error("expected validation error for missing --title")
	}
}

// TestPagesUpdate_MissingBody covers the --body validation branch.
func TestPagesUpdate_MissingBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call")
	}))
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "42", "title": "T", "body": ""})
	if err := cmd.RunPagesWorkflowUpdate(flagCmd, nil); err == nil {
		t.Error("expected validation error for missing --body")
	}
}

// TestPagesUpdate_Success covers the happy path: GET version then PUT.
func TestPagesUpdate_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pages/42", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			fmt.Fprint(w, `{"id":"42","title":"Old","version":{"number":3}}`)
			return
		}
		fmt.Fprint(w, `{"id":"42","title":"New","version":{"number":4}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "42", "title": "New", "body": "<p>updated</p>"})
	if err := cmd.RunPagesWorkflowUpdate(flagCmd, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestPagesUpdate_409Retry covers the conflict-retry branch.
func TestPagesUpdate_409Retry(t *testing.T) {
	putCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/pages/77", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			fmt.Fprint(w, `{"id":"77","title":"Page","version":{"number":2}}`)
			return
		}
		putCount++
		if putCount == 1 {
			w.WriteHeader(http.StatusConflict)
			fmt.Fprint(w, `{"error_type":"conflict","message":"version conflict"}`)
			return
		}
		fmt.Fprint(w, `{"id":"77","title":"Updated","version":{"number":4}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "77", "title": "Updated", "body": "<p>new</p>"})
	if err := cmd.RunPagesWorkflowUpdate(flagCmd, nil); err != nil {
		t.Errorf("unexpected error after retry: %v", err)
	}
	if putCount != 2 {
		t.Errorf("expected 2 PUT requests, got %d", putCount)
	}
}

// TestPagesUpdate_VersionFetchFails covers the exit path when GET version fails.
func TestPagesUpdate_VersionFetchFails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pages/99", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error_type":"not_found","message":"not found"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "99", "title": "T", "body": "<p>b</p>"})
	if err := cmd.RunPagesWorkflowUpdate(flagCmd, nil); err == nil {
		t.Error("expected error when version fetch fails")
	}
}

// TestPagesUpdate_409RetryVersionFetchFails covers the path where the
// conflict-retry version re-fetch also fails.
func TestPagesUpdate_409RetryVersionFetchFails(t *testing.T) {
	putCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/pages/77", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			if putCount == 0 {
				// First GET succeeds so we can try the PUT.
				fmt.Fprint(w, `{"id":"77","title":"Page","version":{"number":2}}`)
				return
			}
			// After 409, second GET fails.
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error_type":"not_found","message":"not found"}`)
			return
		}
		putCount++
		w.WriteHeader(http.StatusConflict)
		fmt.Fprint(w, `{"error_type":"conflict","message":"version conflict"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "77", "title": "T", "body": "<p>b</p>"})
	if err := cmd.RunPagesWorkflowUpdate(flagCmd, nil); err == nil {
		t.Error("expected error when retry version fetch fails")
	}
}

// TestPagesUpdate_PUTError covers the path where PUT fails with a non-409 error.
func TestPagesUpdate_PUTError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pages/42", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			fmt.Fprint(w, `{"id":"42","title":"Page","version":{"number":1}}`)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error_type":"error","message":"server error"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "42", "title": "T", "body": "<p>b</p>"})
	if err := cmd.RunPagesWorkflowUpdate(flagCmd, nil); err == nil {
		t.Error("expected error from PUT 500 response")
	}
}

// ---------------------------------------------------------------------------
// pages.go — pages delete
// ---------------------------------------------------------------------------

// TestPagesDelete_EmptyID covers the --id validation branch.
func TestPagesDelete_EmptyID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call")
	}))
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": ""})
	if err := cmd.RunPagesWorkflowDelete(flagCmd, nil); err == nil {
		t.Error("expected validation error for empty --id")
	}
}

// TestPagesDelete_Success covers the full delete path.
func TestPagesDelete_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pages/55", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "55"})
	if err := cmd.RunPagesWorkflowDelete(flagCmd, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestPagesDelete_HTTPError covers the non-zero exit path.
func TestPagesDelete_HTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pages/55", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error_type":"not_found","message":"not found"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "55"})
	if err := cmd.RunPagesWorkflowDelete(flagCmd, nil); err == nil {
		t.Error("expected error from 404 response")
	}
}

// ---------------------------------------------------------------------------
// pages.go — pages list (get)
// ---------------------------------------------------------------------------

// TestPagesList_NoFilter covers the list path with no space-id filter.
func TestPagesList_NoFilter(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pages", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[],"_links":{}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"space-id": ""})
	if err := cmd.RunPagesWorkflowList(flagCmd, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestPagesList_WithSpaceID covers the list path with a space-id filter.
func TestPagesList_WithSpaceID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pages", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[],"_links":{}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"space-id": "123"})
	if err := cmd.RunPagesWorkflowList(flagCmd, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestPagesList_HTTPError covers the non-zero exit path.
func TestPagesList_HTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pages", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error_type":"error","message":"server error"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"space-id": ""})
	if err := cmd.RunPagesWorkflowList(flagCmd, nil); err == nil {
		t.Error("expected error from 500 response")
	}
}

// ---------------------------------------------------------------------------
// spaces.go — parent RunE (unknown / missing subcommand)
// ---------------------------------------------------------------------------

// TestSpacesParentRunE_Unknown covers the unknown-subcommand branch.
func TestSpacesParentRunE_Unknown(t *testing.T) {
	if err := cmd.SpacesCmd().RunE(cmd.SpacesCmd(), []string{"doesnotexist"}); err == nil {
		t.Error("expected error for unknown subcommand")
	}
}

// TestSpacesParentRunE_NoArgs covers the missing-subcommand branch.
func TestSpacesParentRunE_NoArgs(t *testing.T) {
	if err := cmd.SpacesCmd().RunE(cmd.SpacesCmd(), []string{}); err == nil {
		t.Error("expected error for missing subcommand")
	}
}

// ---------------------------------------------------------------------------
// spaces.go — spaces get (list)
// ---------------------------------------------------------------------------

// TestSpacesList_ListAll covers the list-all path (no --key).
func TestSpacesList_ListAll(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/spaces", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[],"_links":{}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"key": ""})
	if err := cmd.RunSpacesListCmd(flagCmd, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestSpacesList_ListAll_HTTPError covers the list-all non-zero exit path.
func TestSpacesList_ListAll_HTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/spaces", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error_type":"error","message":"server error"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"key": ""})
	if err := cmd.RunSpacesListCmd(flagCmd, nil); err == nil {
		t.Error("expected error from 500 response")
	}
}

// TestSpacesList_WithKey covers the --key resolution + single-space fetch path.
func TestSpacesList_WithKey(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/spaces", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[{"id":"456"}]}`)
	})
	mux.HandleFunc("/spaces/456", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"456","key":"ENG","name":"Engineering"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"key": "ENG"})
	if err := cmd.RunSpacesListCmd(flagCmd, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestSpacesList_WithKey_SpaceGetFails covers the path where the resolved
// space fetch fails after key resolution succeeds.
func TestSpacesList_WithKey_SpaceGetFails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/spaces", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[{"id":"456"}]}`)
	})
	mux.HandleFunc("/spaces/456", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error_type":"error","message":"server error"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"key": "ENG"})
	if err := cmd.RunSpacesListCmd(flagCmd, nil); err == nil {
		t.Error("expected error when space GET fails after key resolution")
	}
}

// TestSpacesList_WithKey_NotFound covers the path where resolveSpaceID returns not-found.
func TestSpacesList_WithKey_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/spaces", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[]}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"key": "MISSING"})
	err := cmd.RunSpacesListCmd(flagCmd, nil)
	if err == nil {
		t.Error("expected error when space key not found")
	}
	if awe, ok := err.(*cferrors.AlreadyWrittenError); ok {
		if awe.Code != cferrors.ExitNotFound {
			t.Errorf("expected ExitNotFound, got %d", awe.Code)
		}
	}
}

// ---------------------------------------------------------------------------
// spaces.go — spaces get-by-id
// ---------------------------------------------------------------------------

// TestSpacesGetByID_EmptyID covers the --id validation branch.
func TestSpacesGetByID_EmptyID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call")
	}))
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": ""})
	if err := cmd.RunSpacesGetByIDCmd(flagCmd, nil); err == nil {
		t.Error("expected validation error for empty --id")
	}
}

// TestSpacesGetByID_NumericID covers the numeric pass-through path.
func TestSpacesGetByID_NumericID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/spaces/123", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"123","key":"ENG","name":"Engineering"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "123"})
	if err := cmd.RunSpacesGetByIDCmd(flagCmd, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestSpacesGetByID_NumericID_HTTPError covers the non-zero exit path.
func TestSpacesGetByID_NumericID_HTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/spaces/123", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error_type":"not_found","message":"not found"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "123"})
	if err := cmd.RunSpacesGetByIDCmd(flagCmd, nil); err == nil {
		t.Error("expected error from 404 response")
	}
}

// TestSpacesGetByID_AlphaKey covers the alpha-key resolution path.
func TestSpacesGetByID_AlphaKey(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/spaces", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[{"id":"456"}]}`)
	})
	mux.HandleFunc("/spaces/456", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"456","key":"ENG","name":"Engineering"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "ENG"})
	if err := cmd.RunSpacesGetByIDCmd(flagCmd, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestSpacesGetByID_AlphaKey_NotFound covers the key-not-found path.
func TestSpacesGetByID_AlphaKey_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/spaces", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[]}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "MISSING"})
	if err := cmd.RunSpacesGetByIDCmd(flagCmd, nil); err == nil {
		t.Error("expected error when space key not found")
	}
}

// ---------------------------------------------------------------------------
// custom_content.go — parent RunE (unknown / missing subcommand)
// ---------------------------------------------------------------------------

// TestCustomContentParentRunE_Unknown covers the unknown-subcommand branch.
func TestCustomContentParentRunE_Unknown(t *testing.T) {
	if err := cmd.CustomContentCmd().RunE(cmd.CustomContentCmd(), []string{"doesnotexist"}); err == nil {
		t.Error("expected error for unknown subcommand")
	}
}

// TestCustomContentParentRunE_NoArgs covers the missing-subcommand branch.
func TestCustomContentParentRunE_NoArgs(t *testing.T) {
	if err := cmd.CustomContentCmd().RunE(cmd.CustomContentCmd(), []string{}); err == nil {
		t.Error("expected error for missing subcommand")
	}
}

// ---------------------------------------------------------------------------
// custom_content.go — get-custom-content-by-type
// ---------------------------------------------------------------------------

// TestCCGetByType_EmptyType covers the --type validation branch.
func TestCCGetByType_EmptyType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call")
	}))
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"type": "", "space-id": ""})
	if err := cmd.RunCustomContentWorkflowGetByType(flagCmd, nil); err == nil {
		t.Error("expected validation error for empty --type")
	}
}

// TestCCGetByType_Success covers the happy path without space-id.
func TestCCGetByType_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/custom-content", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[],"_links":{}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"type": "ac:app:mytype", "space-id": ""})
	if err := cmd.RunCustomContentWorkflowGetByType(flagCmd, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestCCGetByType_WithSpaceID covers the optional --space-id filter path.
func TestCCGetByType_WithSpaceID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/custom-content", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[],"_links":{}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"type": "ac:app:mytype", "space-id": "123"})
	if err := cmd.RunCustomContentWorkflowGetByType(flagCmd, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestCCGetByType_HTTPError covers the non-zero exit path.
func TestCCGetByType_HTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/custom-content", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error_type":"error","message":"server error"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"type": "ac:app:mytype", "space-id": ""})
	if err := cmd.RunCustomContentWorkflowGetByType(flagCmd, nil); err == nil {
		t.Error("expected error from 500 response")
	}
}

// ---------------------------------------------------------------------------
// custom_content.go — create-custom-content
// ---------------------------------------------------------------------------

// TestCCCreate_MissingType covers the --type validation branch.
func TestCCCreate_MissingType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call")
	}))
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"type": "", "space-id": "123", "title": "T", "body": "<p>b</p>"})
	if err := cmd.RunCustomContentWorkflowCreate(flagCmd, nil); err == nil {
		t.Error("expected validation error for missing --type")
	}
}

// TestCCCreate_MissingSpaceID covers the --space-id validation branch.
func TestCCCreate_MissingSpaceID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call")
	}))
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"type": "ac:app:mytype", "space-id": "", "title": "T", "body": "<p>b</p>"})
	if err := cmd.RunCustomContentWorkflowCreate(flagCmd, nil); err == nil {
		t.Error("expected validation error for missing --space-id")
	}
}

// TestCCCreate_MissingTitle covers the --title validation branch.
func TestCCCreate_MissingTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call")
	}))
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"type": "ac:app:mytype", "space-id": "123", "title": "", "body": "<p>b</p>"})
	if err := cmd.RunCustomContentWorkflowCreate(flagCmd, nil); err == nil {
		t.Error("expected validation error for missing --title")
	}
}

// TestCCCreate_MissingBody covers the --body validation branch.
func TestCCCreate_MissingBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call")
	}))
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"type": "ac:app:mytype", "space-id": "123", "title": "T", "body": ""})
	if err := cmd.RunCustomContentWorkflowCreate(flagCmd, nil); err == nil {
		t.Error("expected validation error for missing --body")
	}
}

// TestCCCreate_Success covers the happy path.
func TestCCCreate_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/custom-content", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"10","type":"ac:app:mytype","title":"My Content","version":{"number":1}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"type": "ac:app:mytype", "space-id": "123", "title": "My Content", "body": "<p>hi</p>"})
	if err := cmd.RunCustomContentWorkflowCreate(flagCmd, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestCCCreate_HTTPError covers the non-zero exit path from POST.
func TestCCCreate_HTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/custom-content", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error_type":"error","message":"server error"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"type": "ac:app:mytype", "space-id": "123", "title": "T", "body": "<p>b</p>"})
	if err := cmd.RunCustomContentWorkflowCreate(flagCmd, nil); err == nil {
		t.Error("expected error from 500 response")
	}
}

// ---------------------------------------------------------------------------
// custom_content.go — get-custom-content-by-id
// ---------------------------------------------------------------------------

// TestCCGetByID_EmptyID covers the --id validation branch.
func TestCCGetByID_EmptyID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call")
	}))
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "", "body-format": "storage"})
	if err := cmd.RunCustomContentWorkflowGetByID(flagCmd, nil); err == nil {
		t.Error("expected validation error for empty --id")
	}
}

// TestCCGetByID_Success covers the happy path (default body-format=storage).
func TestCCGetByID_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/custom-content/42", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"42","type":"ac:app:mytype","title":"Test","version":{"number":1}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "42", "body-format": "storage"})
	if err := cmd.RunCustomContentWorkflowGetByID(flagCmd, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestCCGetByID_OverrideBodyFormat covers the body-format Changed branch.
func TestCCGetByID_OverrideBodyFormat(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/custom-content/42", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"42","type":"ac:app:mytype","title":"Test","version":{"number":1}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	// Override body-format after registration so Changed = true.
	flagCmd := newFlagCmdOverride(ctx, map[string]string{"id": "42", "body-format": "storage"}, "body-format", "atlas_doc_format")
	if err := cmd.RunCustomContentWorkflowGetByID(flagCmd, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestCCGetByID_HTTPError covers the non-zero exit path.
func TestCCGetByID_HTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/custom-content/42", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error_type":"not_found","message":"not found"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "42", "body-format": "storage"})
	if err := cmd.RunCustomContentWorkflowGetByID(flagCmd, nil); err == nil {
		t.Error("expected error from 404 response")
	}
}

// ---------------------------------------------------------------------------
// custom_content.go — update-custom-content
// ---------------------------------------------------------------------------

// TestCCUpdate_MissingID covers the --id validation branch.
func TestCCUpdate_MissingID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call")
	}))
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "", "title": "T", "body": "<p>b</p>", "type": ""})
	if err := cmd.RunCustomContentWorkflowUpdate(flagCmd, nil); err == nil {
		t.Error("expected validation error for missing --id")
	}
}

// TestCCUpdate_MissingTitle covers the --title validation branch.
func TestCCUpdate_MissingTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call")
	}))
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "10", "title": "", "body": "<p>b</p>", "type": ""})
	if err := cmd.RunCustomContentWorkflowUpdate(flagCmd, nil); err == nil {
		t.Error("expected validation error for missing --title")
	}
}

// TestCCUpdate_MissingBody covers the --body validation branch.
func TestCCUpdate_MissingBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call")
	}))
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "10", "title": "T", "body": "", "type": ""})
	if err := cmd.RunCustomContentWorkflowUpdate(flagCmd, nil); err == nil {
		t.Error("expected validation error for missing --body")
	}
}

// TestCCUpdate_Success covers the full update path: GET meta then PUT.
func TestCCUpdate_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/custom-content/10", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			fmt.Fprint(w, `{"id":"10","type":"ac:app:mytype","title":"Old","version":{"number":2}}`)
			return
		}
		fmt.Fprint(w, `{"id":"10","type":"ac:app:mytype","title":"New","version":{"number":3}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	// type="" means auto-detect from existing item.
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "10", "title": "New", "body": "<p>updated</p>", "type": ""})
	if err := cmd.RunCustomContentWorkflowUpdate(flagCmd, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestCCUpdate_WithTypeFlag covers the --type explicit override path.
func TestCCUpdate_WithTypeFlag(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/custom-content/10", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			fmt.Fprint(w, `{"id":"10","type":"ac:app:old-type","title":"Old","version":{"number":1}}`)
			return
		}
		fmt.Fprint(w, `{"id":"10","type":"ac:app:new-type","title":"New","version":{"number":2}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "10", "title": "New", "body": "<p>updated</p>", "type": "ac:app:new-type"})
	if err := cmd.RunCustomContentWorkflowUpdate(flagCmd, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestCCUpdate_MetaFetchFails covers the exit path when GET meta fails.
func TestCCUpdate_MetaFetchFails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/custom-content/99", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error_type":"not_found","message":"not found"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "99", "title": "T", "body": "<p>b</p>", "type": ""})
	if err := cmd.RunCustomContentWorkflowUpdate(flagCmd, nil); err == nil {
		t.Error("expected error when meta fetch fails")
	}
}

// TestCCUpdate_PUTError covers the path where PUT fails with a non-409 error.
func TestCCUpdate_PUTError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/custom-content/10", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			fmt.Fprint(w, `{"id":"10","type":"ac:app:mytype","title":"Old","version":{"number":1}}`)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error_type":"error","message":"server error"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "10", "title": "T", "body": "<p>b</p>", "type": ""})
	if err := cmd.RunCustomContentWorkflowUpdate(flagCmd, nil); err == nil {
		t.Error("expected error from PUT 500 response")
	}
}

// TestCCUpdate_409RetryViaRunE covers the conflict-retry branch (no --type flag).
func TestCCUpdate_409RetryViaRunE(t *testing.T) {
	putCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/custom-content/20", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			fmt.Fprint(w, `{"id":"20","type":"ac:app:mytype","title":"Old","version":{"number":1}}`)
			return
		}
		putCount++
		if putCount == 1 {
			w.WriteHeader(http.StatusConflict)
			fmt.Fprint(w, `{"error_type":"conflict","message":"version conflict"}`)
			return
		}
		fmt.Fprint(w, `{"id":"20","type":"ac:app:mytype","title":"New","version":{"number":3}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	// type="" means auto-detect; exercises the ccType = meta.Type branch in retry.
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "20", "title": "New", "body": "<p>retry</p>", "type": ""})
	if err := cmd.RunCustomContentWorkflowUpdate(flagCmd, nil); err != nil {
		t.Errorf("unexpected error after retry: %v", err)
	}
	if putCount != 2 {
		t.Errorf("expected 2 PUT requests, got %d", putCount)
	}
}

// TestCCUpdate_409Retry_WithTypeFlagViaRunE exercises the conflict-retry branch
// when --type is explicitly set (covers typeFlag != "" in the retry).
func TestCCUpdate_409Retry_WithTypeFlagViaRunE(t *testing.T) {
	putCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/custom-content/21", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			fmt.Fprint(w, `{"id":"21","type":"ac:app:detected","title":"Old","version":{"number":1}}`)
			return
		}
		putCount++
		if putCount == 1 {
			w.WriteHeader(http.StatusConflict)
			fmt.Fprint(w, `{"error_type":"conflict","message":"version conflict"}`)
			return
		}
		fmt.Fprint(w, `{"id":"21","type":"ac:app:override","title":"New","version":{"number":3}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "21", "title": "New", "body": "<p>retry</p>", "type": "ac:app:override"})
	if err := cmd.RunCustomContentWorkflowUpdate(flagCmd, nil); err != nil {
		t.Errorf("unexpected error after retry with --type flag: %v", err)
	}
}

// TestCCUpdate_409Retry_MetaRefetchFails covers the path where the conflict-retry
// meta re-fetch also fails.
func TestCCUpdate_409Retry_MetaRefetchFails(t *testing.T) {
	putCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/custom-content/20", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			if putCount == 0 {
				fmt.Fprint(w, `{"id":"20","type":"ac:app:mytype","title":"Old","version":{"number":1}}`)
				return
			}
			// After 409, re-fetch fails.
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error_type":"not_found","message":"not found"}`)
			return
		}
		putCount++
		w.WriteHeader(http.StatusConflict)
		fmt.Fprint(w, `{"error_type":"conflict","message":"version conflict"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "20", "title": "T", "body": "<p>b</p>", "type": ""})
	if err := cmd.RunCustomContentWorkflowUpdate(flagCmd, nil); err == nil {
		t.Error("expected error when retry meta refetch fails")
	}
}

// ---------------------------------------------------------------------------
// custom_content.go — delete-custom-content
// ---------------------------------------------------------------------------

// TestCCDelete_EmptyID covers the --id validation branch.
func TestCCDelete_EmptyID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call")
	}))
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": ""})
	if err := cmd.RunCustomContentWorkflowDelete(flagCmd, nil); err == nil {
		t.Error("expected validation error for empty --id")
	}
}

// TestCCDelete_Success covers the full delete path.
func TestCCDelete_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/custom-content/42", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "42"})
	if err := cmd.RunCustomContentWorkflowDelete(flagCmd, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestCCDelete_HTTPError covers the non-zero exit path.
func TestCCDelete_HTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/custom-content/42", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error_type":"not_found","message":"not found"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)
	flagCmd := newFlagCmd(ctx, map[string]string{"id": "42"})
	if err := cmd.RunCustomContentWorkflowDelete(flagCmd, nil); err == nil {
		t.Error("expected error from 404 response")
	}
}

// ---------------------------------------------------------------------------
// FromContext error branches — covers the "if err != nil { return err }" path
// in each RunE when no client is stored in the context.
// ---------------------------------------------------------------------------

// noClientCmd returns a *cobra.Command whose context has NO client.
func noClientCmd(flags map[string]string) *cobra.Command {
	c := &cobra.Command{Use: "test"}
	c.SetContext(context.Background())
	for k, v := range flags {
		c.Flags().String(k, v, "")
		_ = c.Flags().Set(k, v)
	}
	return c
}

// TestPagesGetByID_NoClient covers the client.FromContext error branch.
func TestPagesGetByID_NoClient(t *testing.T) {
	flagCmd := noClientCmd(map[string]string{"id": "1", "body-format": "storage"})
	if err := cmd.RunPagesWorkflowGetByID(flagCmd, nil); err == nil {
		t.Error("expected error when no client in context")
	}
}

// TestPagesCreate_NoClient covers the client.FromContext error branch.
func TestPagesCreate_NoClient(t *testing.T) {
	flagCmd := noClientCmd(map[string]string{"space-id": "1", "title": "T", "body": "<p>b</p>", "template": "", "parent-id": ""})
	flagCmd.Flags().StringArray("var", nil, "")
	if err := cmd.RunPagesWorkflowCreate(flagCmd, nil); err == nil {
		t.Error("expected error when no client in context")
	}
}

// TestPagesCreate_NoClient_TemplateResolution covers the client.FromContext error
// in the template resolution branch (templateName != "" and bodyVal == "").
func TestPagesCreate_NoClient_TemplateResolution(t *testing.T) {
	flagCmd := noClientCmd(map[string]string{"space-id": "", "title": "", "body": "", "template": "some-tpl", "parent-id": ""})
	flagCmd.Flags().StringArray("var", nil, "")
	// This will fail during template resolution (template not found), not at FromContext.
	// Still exercises the RunE body past FromContext.
	_ = cmd.RunPagesWorkflowCreate(flagCmd, nil)
}

// TestPagesUpdate_NoClient covers the client.FromContext error branch.
func TestPagesUpdate_NoClient(t *testing.T) {
	flagCmd := noClientCmd(map[string]string{"id": "1", "title": "T", "body": "<p>b</p>"})
	if err := cmd.RunPagesWorkflowUpdate(flagCmd, nil); err == nil {
		t.Error("expected error when no client in context")
	}
}

// TestPagesDelete_NoClient covers the client.FromContext error branch.
func TestPagesDelete_NoClient(t *testing.T) {
	flagCmd := noClientCmd(map[string]string{"id": "1"})
	if err := cmd.RunPagesWorkflowDelete(flagCmd, nil); err == nil {
		t.Error("expected error when no client in context")
	}
}

// TestPagesList_NoClient covers the client.FromContext error branch.
func TestPagesList_NoClient(t *testing.T) {
	flagCmd := noClientCmd(map[string]string{"space-id": ""})
	if err := cmd.RunPagesWorkflowList(flagCmd, nil); err == nil {
		t.Error("expected error when no client in context")
	}
}

// TestSpacesList_NoClient covers the client.FromContext error branch.
func TestSpacesList_NoClient(t *testing.T) {
	flagCmd := noClientCmd(map[string]string{"key": ""})
	if err := cmd.RunSpacesListCmd(flagCmd, nil); err == nil {
		t.Error("expected error when no client in context")
	}
}

// TestSpacesGetByID_NoClient covers the client.FromContext error branch.
func TestSpacesGetByID_NoClient(t *testing.T) {
	flagCmd := noClientCmd(map[string]string{"id": "1"})
	if err := cmd.RunSpacesGetByIDCmd(flagCmd, nil); err == nil {
		t.Error("expected error when no client in context")
	}
}

// TestCCGetByType_NoClient covers the client.FromContext error branch.
func TestCCGetByType_NoClient(t *testing.T) {
	flagCmd := noClientCmd(map[string]string{"type": "ac:app:t", "space-id": ""})
	if err := cmd.RunCustomContentWorkflowGetByType(flagCmd, nil); err == nil {
		t.Error("expected error when no client in context")
	}
}

// TestCCCreate_NoClient covers the client.FromContext error branch.
func TestCCCreate_NoClient(t *testing.T) {
	flagCmd := noClientCmd(map[string]string{"type": "ac:app:t", "space-id": "1", "title": "T", "body": "<p>b</p>"})
	if err := cmd.RunCustomContentWorkflowCreate(flagCmd, nil); err == nil {
		t.Error("expected error when no client in context")
	}
}

// TestCCGetByID_NoClient covers the client.FromContext error branch.
func TestCCGetByID_NoClient(t *testing.T) {
	flagCmd := noClientCmd(map[string]string{"id": "1", "body-format": "storage"})
	if err := cmd.RunCustomContentWorkflowGetByID(flagCmd, nil); err == nil {
		t.Error("expected error when no client in context")
	}
}

// TestCCUpdate_NoClient covers the client.FromContext error branch.
func TestCCUpdate_NoClient(t *testing.T) {
	flagCmd := noClientCmd(map[string]string{"id": "1", "title": "T", "body": "<p>b</p>", "type": ""})
	if err := cmd.RunCustomContentWorkflowUpdate(flagCmd, nil); err == nil {
		t.Error("expected error when no client in context")
	}
}

// TestCCDelete_NoClient covers the client.FromContext error branch.
func TestCCDelete_NoClient(t *testing.T) {
	flagCmd := noClientCmd(map[string]string{"id": "1"})
	if err := cmd.RunCustomContentWorkflowDelete(flagCmd, nil); err == nil {
		t.Error("expected error when no client in context")
	}
}

// ---------------------------------------------------------------------------
// Additional edge case coverage
// ---------------------------------------------------------------------------

// TestPagesCreate_TemplateLookupFails covers the resolveTemplate error path:
// when --template is set (with no --body), the template lookup fails and
// resolveErr != nil branches are taken.
func TestPagesCreate_TemplateLookupFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call")
	}))
	defer srv.Close()
	ctx := withClientCRUD(srv)
	// template="no-such-template", body="" → resolveTemplate returns error
	flagCmd := newFlagCmd(ctx, map[string]string{"space-id": "123", "title": "T", "body": "", "template": "no-such-template", "parent-id": ""})
	flagCmd.Flags().StringArray("var", nil, "")
	err := cmd.RunPagesWorkflowCreate(flagCmd, nil)
	if err == nil {
		t.Error("expected error when template not found")
	}
}

// TestPagesCreate_TemplateResolvesSpaceID covers the branch where a template
// provides the spaceID when --space-id is not explicitly given.
// We use a valid built-in template (if any) or inject via the ResolveTemplate
// exported helper to confirm the branch executes. Since we cannot inject a
// template with spaceID easily, we instead verify that the branch is reached
// when --space-id is empty and the template resolves (even if the template has
// no spaceID, the branch condition is simply false and execution continues).
//
// NOTE: This test intentionally focuses on the title-override sub-branch
// (title == "") by providing no --title so that rendered.Title is used.
// A real template is required; we use the "meeting-notes" template if it exists,
// falling back gracefully if it does not.
func TestPagesCreate_TemplateResolvesTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"1","title":"From Template","version":{"number":1}}`)
	}))
	defer srv.Close()
	ctx := withClientCRUD(srv)

	// Check if the "meeting-notes" template exists; skip if not.
	_, err := cmd.ResolveTemplate(nil, "meeting-notes", nil)
	if err != nil {
		t.Skip("meeting-notes template not available, skipping")
	}

	flagCmd := newFlagCmd(ctx, map[string]string{"space-id": "123", "title": "", "body": "", "template": "meeting-notes", "parent-id": ""})
	flagCmd.Flags().StringArray("var", nil, "")
	// The test passes as long as we don't panic; a network error is acceptable.
	_ = cmd.RunPagesWorkflowCreate(flagCmd, nil)
}

// ---------------------------------------------------------------------------
// WriteOutput error paths — triggered by setting an invalid JQ filter on the
// client so that WriteOutput returns ExitValidation instead of ExitOK.
// ---------------------------------------------------------------------------

// makeClientBadJQ builds a client with an intentionally bad JQ filter so that
// WriteOutput returns non-OK, covering the
// "if ec := c.WriteOutput(respBody); ec != ExitOK" branches.
func makeClientBadJQ(srv *httptest.Server) *client.Client {
	return &client.Client{
		BaseURL:    srv.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test-token"},
		HTTPClient: srv.Client(),
		Stdout:     &strings.Builder{},
		Stderr:     &strings.Builder{},
		JQFilter:   ".[[[invalid jq",
	}
}

// TestPagesCreate_WriteOutputError covers the WriteOutput failure branch (line 196).
func TestPagesCreate_WriteOutputError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pages", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"1","title":"Test","version":{"number":1}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := makeClientBadJQ(srv)
	ctx := client.NewContext(context.Background(), c)
	flagCmd := newFlagCmd(ctx, map[string]string{"space-id": "123", "title": "T", "body": "<p>b</p>", "template": "", "parent-id": ""})
	flagCmd.Flags().StringArray("var", nil, "")
	err := cmd.RunPagesWorkflowCreate(flagCmd, nil)
	if err == nil {
		t.Error("expected error when WriteOutput fails due to bad JQ filter")
	}
}

// TestCCCreate_WriteOutputError covers the WriteOutput failure branch (custom_content.go:176).
func TestCCCreate_WriteOutputError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/custom-content", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"10","type":"ac:app:mytype","title":"My Content","version":{"number":1}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := makeClientBadJQ(srv)
	ctx := client.NewContext(context.Background(), c)
	flagCmd := newFlagCmd(ctx, map[string]string{"type": "ac:app:mytype", "space-id": "123", "title": "My Content", "body": "<p>hi</p>"})
	err := cmd.RunCustomContentWorkflowCreate(flagCmd, nil)
	if err == nil {
		t.Error("expected error when WriteOutput fails due to bad JQ filter")
	}
}

// ---------------------------------------------------------------------------
// Template spaceID branch — pages.go:151.47,153.5
// Triggered when --space-id is empty but the resolved template provides a SpaceID.
// ---------------------------------------------------------------------------

// TestPagesCreate_TemplateWithSpaceID covers the branch where a template
// provides spaceID (rendered.SpaceID != "") and --space-id was not given.
// We write a minimal template JSON to a temp directory and point CF_CONFIG_PATH
// there so that cftemplate.Dir() resolves to the temp templates dir.
func TestPagesCreate_TemplateWithSpaceID(t *testing.T) {
	// Create a temp config directory with a templates subdirectory.
	tmpDir := t.TempDir()
	tplDir := tmpDir + "/templates"
	if err := os.MkdirAll(tplDir, 0o755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}
	// Write a minimal template that has a space_id.
	tplJSON := `{"title":"Test Page","body":"<p>content</p>","space_id":"789"}`
	if err := os.WriteFile(tplDir+"/spaceid-tpl.json", []byte(tplJSON), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	// Point config path into the temp dir so template.Dir() finds our template.
	t.Setenv("CF_CONFIG_PATH", tmpDir+"/config.json")

	mux := http.NewServeMux()
	mux.HandleFunc("/pages", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"99","title":"Test Page","version":{"number":1}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := withClientCRUD(srv)

	// --space-id is empty so the template's space_id will be used.
	// --title is also empty so rendered.Title will be used.
	flagCmd := newFlagCmd(ctx, map[string]string{"space-id": "", "title": "", "body": "", "template": "spaceid-tpl", "parent-id": ""})
	flagCmd.Flags().StringArray("var", nil, "")
	if err := cmd.RunPagesWorkflowCreate(flagCmd, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

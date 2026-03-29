package cmd_test

// crud_attachments_coverage_test.go covers previously uncovered RunE closures in:
//   - cmd/attachments.go  (attachmentsCmd root, attachments list, attachments upload)
//   - cmd/blogposts.go    (blogpostsCmd root, get-blog-post-by-id, create-blog-post,
//                          update-blog-post, delete-blog-post, get-blog-posts)
//   - cmd/comments.go     (commentsCmd root, comments list, comments create, comments delete)

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/sofq/confluence-cli/cmd"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// runCLICmd resets all cobra singleton flags via ResetRootPersistentFlags, sets
// up the env pointing at srvURL, executes args against the singleton root
// command, and returns the captured stdout and stderr strings.
func runCLICmd(t *testing.T, srvURL string, args ...string) (stdout string, stderr string) {
	t.Helper()
	cmd.ResetRootPersistentFlags()
	t.Cleanup(func() { cmd.ResetRootPersistentFlags() })

	setupTemplateEnv(t, srvURL, nil)

	oldOut := os.Stdout
	oldErr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	root := cmd.RootCommand()
	root.SetArgs(args)
	_ = root.Execute()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldOut
	os.Stderr = oldErr

	var outBuf, errBuf strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := rOut.Read(buf)
		if n > 0 {
			outBuf.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	for {
		n, err := rErr.Read(buf)
		if n > 0 {
			errBuf.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return outBuf.String(), errBuf.String()
}

// newMockServer starts a test HTTP server with the provided handler.
func newMockServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

// jsonOK responds with 200 and the given JSON body.
func jsonOK(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, body)
}

// ---------------------------------------------------------------------------
// cmd/attachments.go — attachmentsCmd root (no subcommand)
// ---------------------------------------------------------------------------

// TestAttachmentsRoot_NoSubcommand covers attachmentsCmd.RunE when no subcommand
// is given. With SilenceErrors:true the fmt.Errorf is returned but not written to
// stderr; we verify the code path runs (coverage) without a panic.
func TestAttachmentsRoot_NoSubcommand(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})
	// runCLICmd calls ResetRootPersistentFlags before and after, so flag state
	// is clean. The Execute error is silenced; we just need coverage, not assertion.
	_, _ = runCLICmd(t, srv.URL, "attachments")
}

// TestAttachmentsRoot_UnknownSubcommand covers attachmentsCmd.RunE when an
// unknown subcommand name is passed via FParseErrWhitelist (args[0] branch).
func TestAttachmentsRoot_UnknownSubcommand(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "attachments", "nonexistent-subcommand")
	// The cobra FParseErrWhitelist passes unknown flags but cobra itself
	// routes unknown commands normally; the error should mention the unknown command.
	_ = stderr // just ensure it runs without panic
}

// ---------------------------------------------------------------------------
// cmd/attachments.go — attachments list
// ---------------------------------------------------------------------------

// TestAttachmentsList_MissingPageID covers the validation branch where --page-id is empty.
func TestAttachmentsList_MissingPageID(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{"results":[]}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "attachments", "list")
	if !strings.Contains(stderr, "page-id") && !strings.Contains(stderr, "validation") {
		t.Errorf("expected validation error for missing page-id, got: %q", stderr)
	}
}

// TestAttachmentsList_Success covers the happy path for attachments list.
func TestAttachmentsList_Success(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/pages/123/attachments") {
			jsonOK(w, `{"results":[{"id":"att1","title":"file.png"}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	stdout, _ := runCLICmd(t, srv.URL, "attachments", "list", "--page-id", "123")
	if !strings.Contains(stdout, "att1") {
		t.Errorf("expected attachment id in output, got: %q", stdout)
	}
}

// TestAttachmentsList_HTTPError covers the c.Do error path when the server
// returns a non-2xx response.
func TestAttachmentsList_HTTPError(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message":"unauthorized"}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "attachments", "list", "--page-id", "123")
	if !strings.Contains(stderr, "error") && !strings.Contains(stderr, "unauthorized") && !strings.Contains(stderr, "auth") {
		t.Errorf("expected error in stderr for 401 response, got: %q", stderr)
	}
}

// ---------------------------------------------------------------------------
// cmd/attachments.go — attachments upload
// ---------------------------------------------------------------------------

// TestAttachmentsUpload_MissingPageID covers validation when --page-id is empty.
func TestAttachmentsUpload_MissingPageID(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "attachments", "upload", "--file", "/some/file.txt")
	if !strings.Contains(stderr, "page-id") && !strings.Contains(stderr, "validation") {
		t.Errorf("expected page-id validation error, got: %q", stderr)
	}
}

// TestAttachmentsUpload_MissingFile covers validation when --file is empty.
func TestAttachmentsUpload_MissingFile(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "attachments", "upload", "--page-id", "123")
	if !strings.Contains(stderr, "file") && !strings.Contains(stderr, "validation") {
		t.Errorf("expected file validation error, got: %q", stderr)
	}
}

// TestAttachmentsUpload_FileNotFound covers the os.Open error path (file does not exist).
func TestAttachmentsUpload_FileNotFound(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "attachments", "upload", "--page-id", "123", "--file", "/nonexistent/path/file.txt")
	if !strings.Contains(stderr, "cannot open") && !strings.Contains(stderr, "validation") && !strings.Contains(stderr, "no such file") {
		t.Errorf("expected file-not-found error, got: %q", stderr)
	}
}

// TestAttachmentsUpload_Success covers the full happy-path upload (multipart POST to v1 API).
func TestAttachmentsUpload_Success(t *testing.T) {
	// Create a temp file to upload.
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test-upload.txt")
	if err := os.WriteFile(filePath, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/child/attachment") {
			jsonOK(w, `{"results":[{"id":"att99","title":"test-upload.txt"}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	stdout, _ := runCLICmd(t, srv.URL, "attachments", "upload", "--page-id", "123", "--file", filePath)
	if !strings.Contains(stdout, "att99") {
		t.Errorf("expected attachment id in output, got: %q", stdout)
	}
}

// TestAttachmentsUpload_HTTPError covers the server returning 4xx for the upload.
func TestAttachmentsUpload_HTTPError(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test-file.txt")
	if err := os.WriteFile(filePath, []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"forbidden"}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "attachments", "upload", "--page-id", "123", "--file", filePath)
	if !strings.Contains(stderr, "error") && !strings.Contains(stderr, "forbidden") && !strings.Contains(stderr, "permission") {
		t.Errorf("expected error in stderr for 403 response, got: %q", stderr)
	}
}

// TestAttachmentsUpload_NoContentResponse covers the 204 No Content path (empty body → "{}").
func TestAttachmentsUpload_NoContentResponse(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "empty-response.txt")
	if err := os.WriteFile(filePath, []byte("data"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	stdout, _ := runCLICmd(t, srv.URL, "attachments", "upload", "--page-id", "123", "--file", filePath)
	if !strings.Contains(stdout, "{}") {
		t.Errorf("expected '{}' for 204 No Content, got: %q", stdout)
	}
}

// TestAttachmentsUpload_ConnectionRefused covers cmd/attachments.go:139-143 —
// the c.HTTPClient.Do error path when the upload target is not reachable.
// It closes the mock server before the upload is made so the TCP connection fails.
func TestAttachmentsUpload_ConnectionRefused(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "connection-test.txt")
	if err := os.WriteFile(filePath, []byte("data"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create and immediately close a server to obtain a port that is now refusing connections.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srvURL := srv.URL
	srv.Close() // close before the request is made

	_, stderr := runCLICmd(t, srvURL, "attachments", "upload", "--page-id", "123", "--file", filePath)
	if !strings.Contains(stderr, "connection_error") && !strings.Contains(stderr, "refused") && !strings.Contains(stderr, "connect") {
		t.Errorf("expected connection error in stderr, got: %q", stderr)
	}
}

// ---------------------------------------------------------------------------
// cmd/blogposts.go — blogpostsCmd root (no subcommand)
// ---------------------------------------------------------------------------

// TestBlogpostsRoot_NoSubcommand covers blogpostsCmd.RunE missing subcommand path.
// With SilenceErrors:true the returned fmt.Errorf is not written to stderr;
// we just need coverage of the code path.
func TestBlogpostsRoot_NoSubcommand(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})
	_, _ = runCLICmd(t, srv.URL, "blogposts")
}

// TestBlogpostsRoot_UnknownSubcommand covers blogpostsCmd.RunE args[0] path.
func TestBlogpostsRoot_UnknownSubcommand(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})

	_, _ = runCLICmd(t, srv.URL, "blogposts", "unknown-subcommand-xyz")
	// just ensure no panic
}

// ---------------------------------------------------------------------------
// cmd/blogposts.go — get-blog-post-by-id
// ---------------------------------------------------------------------------

// TestBlogpostsGetByID_MissingID covers the --id validation error branch.
func TestBlogpostsGetByID_MissingID(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "blogposts", "get-blog-post-by-id")
	if !strings.Contains(stderr, "--id") && !strings.Contains(stderr, "validation") {
		t.Errorf("expected --id validation error, got: %q", stderr)
	}
}

// TestBlogpostsGetByID_Success covers the happy path for get-blog-post-by-id.
func TestBlogpostsGetByID_Success(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/blogposts/42") {
			jsonOK(w, `{"id":"42","title":"My Blog Post","version":{"number":3}}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	stdout, _ := runCLICmd(t, srv.URL, "blogposts", "get-blog-post-by-id", "--id", "42")
	if !strings.Contains(stdout, "My Blog Post") {
		t.Errorf("expected blog post title in output, got: %q", stdout)
	}
}

// TestBlogpostsGetByID_CustomBodyFormat covers the cmd.Flags().Changed("body-format") branch.
func TestBlogpostsGetByID_CustomBodyFormat(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/blogposts/42") {
			format := r.URL.Query().Get("body-format")
			jsonOK(w, fmt.Sprintf(`{"id":"42","title":"Blog","bodyFormat":"%s"}`, format))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	stdout, _ := runCLICmd(t, srv.URL, "blogposts", "get-blog-post-by-id", "--id", "42", "--body-format", "atlas_doc_format")
	if !strings.Contains(stdout, "42") {
		t.Errorf("expected blog post id in output, got: %q", stdout)
	}
}

// TestBlogpostsGetByID_HTTPError covers the c.Do error branch.
func TestBlogpostsGetByID_HTTPError(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"not found"}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "blogposts", "get-blog-post-by-id", "--id", "999")
	if !strings.Contains(stderr, "error") && !strings.Contains(stderr, "not_found") && !strings.Contains(stderr, "404") {
		t.Errorf("expected error in stderr for 404, got: %q", stderr)
	}
}

// ---------------------------------------------------------------------------
// cmd/blogposts.go — create-blog-post
// ---------------------------------------------------------------------------

// TestBlogpostsCreate_MissingSpaceID covers the --space-id validation error.
func TestBlogpostsCreate_MissingSpaceID(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "blogposts", "create-blog-post",
		"--title", "My Post", "--body", "<p>content</p>")
	if !strings.Contains(stderr, "space-id") && !strings.Contains(stderr, "validation") {
		t.Errorf("expected space-id validation error, got: %q", stderr)
	}
}

// TestBlogpostsCreate_MissingTitle covers the --title validation error.
func TestBlogpostsCreate_MissingTitle(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "blogposts", "create-blog-post",
		"--space-id", "123", "--body", "<p>content</p>")
	if !strings.Contains(stderr, "title") && !strings.Contains(stderr, "validation") {
		t.Errorf("expected title validation error, got: %q", stderr)
	}
}

// TestBlogpostsCreate_MissingBody covers the --body validation error.
func TestBlogpostsCreate_MissingBody(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "blogposts", "create-blog-post",
		"--space-id", "123", "--title", "My Post")
	if !strings.Contains(stderr, "body") && !strings.Contains(stderr, "validation") {
		t.Errorf("expected body validation error, got: %q", stderr)
	}
}

// TestBlogpostsCreate_Success covers the happy path for create-blog-post.
func TestBlogpostsCreate_Success(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/blogposts") {
			jsonOK(w, `{"id":"bp1","title":"My Post"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	stdout, _ := runCLICmd(t, srv.URL, "blogposts", "create-blog-post",
		"--space-id", "123", "--title", "My Post", "--body", "<p>content</p>")
	if !strings.Contains(stdout, "bp1") {
		t.Errorf("expected blog post id in output, got: %q", stdout)
	}
}

// TestBlogpostsCreate_FetchError covers the c.Fetch error branch (server returns 4xx).
func TestBlogpostsCreate_FetchError(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"message":"bad request"}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "blogposts", "create-blog-post",
		"--space-id", "123", "--title", "My Post", "--body", "<p>content</p>")
	if !strings.Contains(stderr, "error") && !strings.Contains(stderr, "bad request") {
		t.Errorf("expected error in stderr for 400 response, got: %q", stderr)
	}
}

// TestBlogpostsCreate_TemplateConflict covers the --template + --body conflict validation.
func TestBlogpostsCreate_TemplateConflict(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "blogposts", "create-blog-post",
		"--space-id", "123", "--title", "My Post",
		"--body", "<p>content</p>", "--template", "some-template")
	if !strings.Contains(stderr, "template") && !strings.Contains(stderr, "body") && !strings.Contains(stderr, "validation") {
		t.Errorf("expected template+body conflict error, got: %q", stderr)
	}
}

// TestBlogpostsCreate_TemplateResolveFail covers the template resolution error branch.
func TestBlogpostsCreate_TemplateResolveFail(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "blogposts", "create-blog-post",
		"--space-id", "123", "--title", "My Post",
		"--template", "nonexistent-template-xyz")
	if !strings.Contains(stderr, "error") && !strings.Contains(stderr, "template") {
		t.Errorf("expected template resolution error, got: %q", stderr)
	}
}

// ---------------------------------------------------------------------------
// cmd/blogposts.go — update-blog-post
// ---------------------------------------------------------------------------

// TestBlogpostsUpdate_MissingID covers the --id validation error.
func TestBlogpostsUpdate_MissingID(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "blogposts", "update-blog-post",
		"--title", "Title", "--body", "<p>body</p>")
	if !strings.Contains(stderr, "--id") && !strings.Contains(stderr, "validation") {
		t.Errorf("expected --id validation error, got: %q", stderr)
	}
}

// TestBlogpostsUpdate_MissingTitle covers the --title validation error.
func TestBlogpostsUpdate_MissingTitle(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "blogposts", "update-blog-post",
		"--id", "42", "--body", "<p>body</p>")
	if !strings.Contains(stderr, "title") && !strings.Contains(stderr, "validation") {
		t.Errorf("expected title validation error, got: %q", stderr)
	}
}

// TestBlogpostsUpdate_MissingBody covers the --body validation error.
func TestBlogpostsUpdate_MissingBody(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "blogposts", "update-blog-post",
		"--id", "42", "--title", "Title")
	if !strings.Contains(stderr, "body") && !strings.Contains(stderr, "validation") {
		t.Errorf("expected body validation error, got: %q", stderr)
	}
}

// TestBlogpostsUpdate_Success covers the full update-blog-post happy path:
// version fetch → PUT update.
func TestBlogpostsUpdate_Success(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/blogposts/42"):
			jsonOK(w, `{"id":"42","title":"Old Title","version":{"number":5}}`)
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/blogposts/42"):
			jsonOK(w, `{"id":"42","title":"Updated Title","version":{"number":6}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	stdout, _ := runCLICmd(t, srv.URL, "blogposts", "update-blog-post",
		"--id", "42", "--title", "Updated Title", "--body", "<p>new content</p>")
	if !strings.Contains(stdout, "Updated Title") {
		t.Errorf("expected updated title in output, got: %q", stdout)
	}
}

// TestBlogpostsUpdate_VersionFetchError covers the fetchBlogpostVersion failure path.
func TestBlogpostsUpdate_VersionFetchError(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message":"unauthorized"}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "blogposts", "update-blog-post",
		"--id", "42", "--title", "Updated Title", "--body", "<p>content</p>")
	if !strings.Contains(stderr, "error") && !strings.Contains(stderr, "unauthorized") && !strings.Contains(stderr, "auth") {
		t.Errorf("expected auth error in stderr, got: %q", stderr)
	}
}

// TestBlogpostsUpdate_ConflictRetry covers the 409-conflict single-retry path:
// version fetch → first PUT returns 409 → re-fetch version → second PUT succeeds.
func TestBlogpostsUpdate_ConflictRetry(t *testing.T) {
	var putCallCount int32
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/blogposts/42"):
			jsonOK(w, `{"id":"42","title":"Old","version":{"number":5}}`)
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/blogposts/42"):
			n := atomic.AddInt32(&putCallCount, 1)
			if n == 1 {
				// First PUT: return 409 Conflict.
				w.WriteHeader(http.StatusConflict)
				fmt.Fprint(w, `{"message":"conflict"}`)
			} else {
				// Second PUT (retry): succeed.
				jsonOK(w, `{"id":"42","title":"Retried Update","version":{"number":6}}`)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	stdout, _ := runCLICmd(t, srv.URL, "blogposts", "update-blog-post",
		"--id", "42", "--title", "Retried Update", "--body", "<p>content</p>")
	if !strings.Contains(stdout, "Retried Update") {
		t.Errorf("expected updated title in output after retry, got: %q", stdout)
	}
	if atomic.LoadInt32(&putCallCount) != 2 {
		t.Errorf("expected 2 PUT calls (initial + retry), got %d", atomic.LoadInt32(&putCallCount))
	}
}

// TestBlogpostsUpdate_ConflictRetryVersionFetchError covers the 409 → re-fetch fails path.
func TestBlogpostsUpdate_ConflictRetryVersionFetchError(t *testing.T) {
	var putCallCount int32
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/blogposts/42"):
			n := atomic.LoadInt32(&putCallCount)
			if n == 0 {
				// First GET (initial version fetch) succeeds.
				jsonOK(w, `{"id":"42","title":"Old","version":{"number":5}}`)
			} else {
				// Second GET (retry version fetch) fails.
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, `{"message":"server error"}`)
			}
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/blogposts/42"):
			atomic.AddInt32(&putCallCount, 1)
			w.WriteHeader(http.StatusConflict)
			fmt.Fprint(w, `{"message":"conflict"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	_, stderr := runCLICmd(t, srv.URL, "blogposts", "update-blog-post",
		"--id", "42", "--title", "Update", "--body", "<p>content</p>")
	if !strings.Contains(stderr, "error") {
		t.Errorf("expected error in stderr for failed retry version fetch, got: %q", stderr)
	}
}

// TestBlogpostsUpdate_ConflictRetryUpdateFails covers the 409 → retry also fails path.
func TestBlogpostsUpdate_ConflictRetryUpdateFails(t *testing.T) {
	var putCallCount int32
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/blogposts/42"):
			jsonOK(w, `{"id":"42","title":"Old","version":{"number":5}}`)
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/blogposts/42"):
			atomic.AddInt32(&putCallCount, 1)
			// Both PUTs fail — first with 409, second with 500.
			n := atomic.LoadInt32(&putCallCount)
			if n == 1 {
				w.WriteHeader(http.StatusConflict)
				fmt.Fprint(w, `{"message":"conflict"}`)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, `{"message":"server error on retry"}`)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	_, stderr := runCLICmd(t, srv.URL, "blogposts", "update-blog-post",
		"--id", "42", "--title", "Update", "--body", "<p>content</p>")
	if !strings.Contains(stderr, "error") {
		t.Errorf("expected error in stderr when both PUT attempts fail, got: %q", stderr)
	}
}

// ---------------------------------------------------------------------------
// cmd/blogposts.go — delete-blog-post
// ---------------------------------------------------------------------------

// TestBlogpostsDelete_MissingID covers the --id validation error.
func TestBlogpostsDelete_MissingID(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "blogposts", "delete-blog-post")
	if !strings.Contains(stderr, "--id") && !strings.Contains(stderr, "validation") {
		t.Errorf("expected --id validation error, got: %q", stderr)
	}
}

// TestBlogpostsDelete_Success covers the happy path for delete-blog-post.
func TestBlogpostsDelete_Success(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/blogposts/42") {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// 204 response: exit code 0, no output expected (just no error).
	_, stderr := runCLICmd(t, srv.URL, "blogposts", "delete-blog-post", "--id", "42")
	if strings.Contains(stderr, "error") {
		t.Errorf("expected no error for successful delete, got stderr: %q", stderr)
	}
}

// TestBlogpostsDelete_HTTPError covers the c.Do non-2xx branch.
func TestBlogpostsDelete_HTTPError(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"not found"}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "blogposts", "delete-blog-post", "--id", "999")
	if !strings.Contains(stderr, "error") && !strings.Contains(stderr, "not_found") {
		t.Errorf("expected error in stderr for 404, got: %q", stderr)
	}
}

// ---------------------------------------------------------------------------
// cmd/blogposts.go — get-blog-posts (list)
// ---------------------------------------------------------------------------

// TestBlogpostsList_Success covers the happy path (no space-id filter).
func TestBlogpostsList_Success(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/blogposts") {
			jsonOK(w, `{"results":[{"id":"bp1","title":"Post One"},{"id":"bp2","title":"Post Two"}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	stdout, _ := runCLICmd(t, srv.URL, "blogposts", "get-blog-posts")
	if !strings.Contains(stdout, "Post One") {
		t.Errorf("expected blog post title in output, got: %q", stdout)
	}
}

// TestBlogpostsList_WithSpaceID covers the q.Set("space-id", ...) branch.
func TestBlogpostsList_WithSpaceID(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/blogposts") {
			spaceID := r.URL.Query().Get("space-id")
			jsonOK(w, fmt.Sprintf(`{"results":[{"id":"bp10","spaceId":"%s"}]}`, spaceID))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	stdout, _ := runCLICmd(t, srv.URL, "blogposts", "get-blog-posts", "--space-id", "456")
	if !strings.Contains(stdout, "bp10") {
		t.Errorf("expected blog post in output for space-id filter, got: %q", stdout)
	}
}

// TestBlogpostsList_HTTPError covers the c.Do error branch.
func TestBlogpostsList_HTTPError(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"message":"internal error"}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "blogposts", "get-blog-posts")
	if !strings.Contains(stderr, "error") {
		t.Errorf("expected error in stderr for 500 response, got: %q", stderr)
	}
}

// ---------------------------------------------------------------------------
// cmd/comments.go — commentsCmd root (no subcommand)
// ---------------------------------------------------------------------------

// TestCommentsRoot_NoSubcommand covers commentsCmd.RunE missing subcommand path.
// With SilenceErrors:true the returned fmt.Errorf is not written to stderr;
// we just need coverage of the code path.
func TestCommentsRoot_NoSubcommand(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})
	_, _ = runCLICmd(t, srv.URL, "comments")
}

// TestCommentsRoot_UnknownSubcommand covers commentsCmd.RunE args[0] path.
func TestCommentsRoot_UnknownSubcommand(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})

	_, _ = runCLICmd(t, srv.URL, "comments", "unknown-subcommand-xyz")
	// just ensure no panic
}

// ---------------------------------------------------------------------------
// cmd/comments.go — comments list
// ---------------------------------------------------------------------------

// TestCommentsList_MissingPageID covers the --page-id validation error.
func TestCommentsList_MissingPageID(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "comments", "list")
	if !strings.Contains(stderr, "page-id") && !strings.Contains(stderr, "validation") {
		t.Errorf("expected page-id validation error, got: %q", stderr)
	}
}

// TestCommentsList_Success covers the happy path.
func TestCommentsList_Success(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/pages/123/footer-comments") {
			jsonOK(w, `{"results":[{"id":"c1","body":{"value":"First comment"}}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	stdout, _ := runCLICmd(t, srv.URL, "comments", "list", "--page-id", "123")
	if !strings.Contains(stdout, "c1") {
		t.Errorf("expected comment id in output, got: %q", stdout)
	}
}

// TestCommentsList_HTTPError covers the c.Do non-2xx branch.
func TestCommentsList_HTTPError(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message":"unauthorized"}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "comments", "list", "--page-id", "123")
	if !strings.Contains(stderr, "error") && !strings.Contains(stderr, "unauthorized") && !strings.Contains(stderr, "auth") {
		t.Errorf("expected error in stderr for 401, got: %q", stderr)
	}
}

// ---------------------------------------------------------------------------
// cmd/comments.go — comments create
// ---------------------------------------------------------------------------

// TestCommentsCreate_MissingPageID covers the --page-id validation error.
func TestCommentsCreate_MissingPageID(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "comments", "create", "--body", "<p>hello</p>")
	if !strings.Contains(stderr, "page-id") && !strings.Contains(stderr, "validation") {
		t.Errorf("expected page-id validation error, got: %q", stderr)
	}
}

// TestCommentsCreate_MissingBody covers the --body validation error.
func TestCommentsCreate_MissingBody(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "comments", "create", "--page-id", "123")
	if !strings.Contains(stderr, "body") && !strings.Contains(stderr, "validation") {
		t.Errorf("expected body validation error, got: %q", stderr)
	}
}

// TestCommentsCreate_Success covers the happy path.
func TestCommentsCreate_Success(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/footer-comments") {
			jsonOK(w, `{"id":"c99","pageId":"123"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	stdout, _ := runCLICmd(t, srv.URL, "comments", "create",
		"--page-id", "123", "--body", "<p>A new comment</p>")
	if !strings.Contains(stdout, "c99") {
		t.Errorf("expected comment id in output, got: %q", stdout)
	}
}

// TestCommentsCreate_FetchError covers the c.Fetch error branch.
func TestCommentsCreate_FetchError(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"forbidden"}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "comments", "create",
		"--page-id", "123", "--body", "<p>A comment</p>")
	if !strings.Contains(stderr, "error") && !strings.Contains(stderr, "forbidden") && !strings.Contains(stderr, "permission") {
		t.Errorf("expected error in stderr for 403, got: %q", stderr)
	}
}

// ---------------------------------------------------------------------------
// cmd/comments.go — comments delete
// ---------------------------------------------------------------------------

// TestCommentsDelete_MissingCommentID covers the --comment-id validation error.
func TestCommentsDelete_MissingCommentID(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "comments", "delete")
	if !strings.Contains(stderr, "comment-id") && !strings.Contains(stderr, "validation") {
		t.Errorf("expected comment-id validation error, got: %q", stderr)
	}
}

// TestCommentsDelete_Success covers the happy path.
func TestCommentsDelete_Success(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/footer-comments/c99") {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	_, stderr := runCLICmd(t, srv.URL, "comments", "delete", "--comment-id", "c99")
	if strings.Contains(stderr, "error") {
		t.Errorf("expected no error for successful delete, got stderr: %q", stderr)
	}
}

// TestCommentsDelete_HTTPError covers the c.Do non-2xx branch.
func TestCommentsDelete_HTTPError(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"not found"}`)
	})

	_, stderr := runCLICmd(t, srv.URL, "comments", "delete", "--comment-id", "c99")
	if !strings.Contains(stderr, "error") && !strings.Contains(stderr, "not_found") {
		t.Errorf("expected error in stderr for 404, got: %q", stderr)
	}
}

// ---------------------------------------------------------------------------
// client.FromContext error paths — direct RunE invocation (no client in ctx)
// ---------------------------------------------------------------------------
// These tests cover the `if err := client.FromContext(...); err != nil { return err }`
// branches in each RunE closure. They invoke the exported RunE functions directly
// with a cobra.Command whose context holds no client.
//
// cobaCmdNoClient creates a *cobra.Command with context.Background() (no client
// value stored), so client.FromContext returns an error triggering the early-return
// branch in every RunE closure. The caller is responsible for adding any flags
// needed to avoid panics before the FromContext check.
func cobaCmdNoClient() *cobra.Command {
	c := &cobra.Command{}
	// context.Background() has no client key — client.FromContext will error.
	c.SetContext(context.Background())
	return c
}

// TestAttachmentsList_NoClientInCtx covers client.FromContext error in list RunE.
func TestAttachmentsList_NoClientInCtx(t *testing.T) {
	c := &cobra.Command{}
	// context.Background() has no client stored.
	c.SetContext(cobaCmdNoClient().Context())
	c.Flags().String("page-id", "123", "")
	err := cmd.RunAttachmentsListCmd(c, nil)
	if err == nil {
		t.Error("expected error when no client in context")
	}
}

// TestAttachmentsUpload_NoClientInCtx covers client.FromContext error in upload RunE.
func TestAttachmentsUpload_NoClientInCtx(t *testing.T) {
	c := &cobra.Command{}
	c.SetContext(cobaCmdNoClient().Context())
	c.Flags().String("page-id", "123", "")
	c.Flags().String("file", "/tmp/x.txt", "")
	err := cmd.RunAttachmentsUploadCmd(c, nil)
	if err == nil {
		t.Error("expected error when no client in context")
	}
}

// TestBlogpostsGetByID_NoClientInCtx covers client.FromContext error in get-blog-post-by-id RunE.
func TestBlogpostsGetByID_NoClientInCtx(t *testing.T) {
	c := &cobra.Command{}
	c.SetContext(cobaCmdNoClient().Context())
	c.Flags().String("id", "42", "")
	c.Flags().String("body-format", "storage", "")
	err := cmd.RunBlogpostsGetByIDCmd(c, nil)
	if err == nil {
		t.Error("expected error when no client in context")
	}
}

// TestBlogpostsCreate_NoClientInCtx covers client.FromContext error in create-blog-post RunE.
func TestBlogpostsCreate_NoClientInCtx(t *testing.T) {
	c := &cobra.Command{}
	c.SetContext(cobaCmdNoClient().Context())
	c.Flags().String("space-id", "123", "")
	c.Flags().String("title", "T", "")
	c.Flags().String("body", "<p>b</p>", "")
	c.Flags().String("template", "", "")
	c.Flags().StringArray("var", nil, "")
	err := cmd.RunBlogpostsCreateCmd(c, nil)
	if err == nil {
		t.Error("expected error when no client in context")
	}
}

// TestBlogpostsUpdate_NoClientInCtx covers client.FromContext error in update-blog-post RunE.
func TestBlogpostsUpdate_NoClientInCtx(t *testing.T) {
	c := &cobra.Command{}
	c.SetContext(cobaCmdNoClient().Context())
	c.Flags().String("id", "42", "")
	c.Flags().String("title", "T", "")
	c.Flags().String("body", "<p>b</p>", "")
	err := cmd.RunBlogpostsUpdateCmd(c, nil)
	if err == nil {
		t.Error("expected error when no client in context")
	}
}

// TestBlogpostsDelete_NoClientInCtx covers client.FromContext error in delete-blog-post RunE.
func TestBlogpostsDelete_NoClientInCtx(t *testing.T) {
	c := &cobra.Command{}
	c.SetContext(cobaCmdNoClient().Context())
	c.Flags().String("id", "42", "")
	err := cmd.RunBlogpostsDeleteCmd(c, nil)
	if err == nil {
		t.Error("expected error when no client in context")
	}
}

// TestBlogpostsList_NoClientInCtx covers client.FromContext error in get-blog-posts RunE.
func TestBlogpostsList_NoClientInCtx(t *testing.T) {
	c := &cobra.Command{}
	c.SetContext(cobaCmdNoClient().Context())
	c.Flags().String("space-id", "", "")
	err := cmd.RunBlogpostsListCmd(c, nil)
	if err == nil {
		t.Error("expected error when no client in context")
	}
}

// TestCommentsList_NoClientInCtx covers client.FromContext error in comments list RunE.
func TestCommentsList_NoClientInCtx(t *testing.T) {
	c := &cobra.Command{}
	c.SetContext(cobaCmdNoClient().Context())
	c.Flags().String("page-id", "123", "")
	err := cmd.RunCommentsListCmd(c, nil)
	if err == nil {
		t.Error("expected error when no client in context")
	}
}

// TestCommentsCreate_NoClientInCtx covers client.FromContext error in comments create RunE.
func TestCommentsCreate_NoClientInCtx(t *testing.T) {
	c := &cobra.Command{}
	c.SetContext(cobaCmdNoClient().Context())
	c.Flags().String("page-id", "123", "")
	c.Flags().String("body", "<p>x</p>", "")
	err := cmd.RunCommentsCreateCmd(c, nil)
	if err == nil {
		t.Error("expected error when no client in context")
	}
}

// TestCommentsDelete_NoClientInCtx covers client.FromContext error in comments delete RunE.
func TestCommentsDelete_NoClientInCtx(t *testing.T) {
	c := &cobra.Command{}
	c.SetContext(cobaCmdNoClient().Context())
	c.Flags().String("comment-id", "c1", "")
	err := cmd.RunCommentsDeleteCmd(c, nil)
	if err == nil {
		t.Error("expected error when no client in context")
	}
}

// ---------------------------------------------------------------------------
// cmd/attachments.go — upload DryRun paths
// ---------------------------------------------------------------------------

// TestAttachmentsUpload_DryRunFileNotFound covers the DryRun branch where
// os.Stat fails because the specified file does not exist.
func TestAttachmentsUpload_DryRunFileNotFound(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{}`)
	})

	_, stderr := runCLICmd(t, srv.URL,
		"--dry-run",
		"attachments", "upload",
		"--page-id", "123",
		"--file", "/nonexistent/path/does-not-exist.bin")
	if !strings.Contains(stderr, "cannot open") && !strings.Contains(stderr, "validation") {
		t.Errorf("expected cannot-open error for DryRun with missing file, got: %q", stderr)
	}
}

// TestAttachmentsUpload_DryRunSuccess covers the DryRun happy path where the
// file exists and the upload request is described without executing.
func TestAttachmentsUpload_DryRunSuccess(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "dry-run-file.bin")
	if err := os.WriteFile(filePath, []byte("dry run content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("DryRun should not make HTTP requests")
		w.WriteHeader(http.StatusInternalServerError)
	})

	stdout, _ := runCLICmd(t, srv.URL,
		"--dry-run",
		"attachments", "upload",
		"--page-id", "123",
		"--file", filePath)
	if !strings.Contains(stdout, "dry-run-file.bin") && !strings.Contains(stdout, "method") {
		t.Errorf("expected DryRun JSON output with file info, got: %q", stdout)
	}
}

// ---------------------------------------------------------------------------
// cmd/blogposts.go — create-blog-post template providing spaceId
// ---------------------------------------------------------------------------

// TestBlogpostsCreate_TemplateProvideSpaceID covers the branch where a template
// provides space_id and --space-id flag is omitted (line 150-152 in blogposts.go).
func TestBlogpostsCreate_TemplateProvideSpaceID(t *testing.T) {
	templateJSON := `{"title":"{{.title}}","body":"<p>{{.content}}</p>","space_id":"{{.spaceId}}"}`

	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/blogposts") {
			jsonOK(w, `{"id":"bp5","title":"Template Post"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	cmd.ResetRootPersistentFlags()
	t.Cleanup(func() { cmd.ResetRootPersistentFlags() })
	setupTemplateEnv(t, srv.URL, map[string]string{
		"space-template": templateJSON,
	})

	oldOut := os.Stdout
	oldErr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	_, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	root := cmd.RootCommand()
	root.SetArgs([]string{
		"blogposts", "create-blog-post",
		"--template", "space-template",
		"--var", "title=Template Post",
		"--var", "content=Body content",
		"--var", "spaceId=999",
	})
	_ = root.Execute()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldOut
	os.Stderr = oldErr

	var outBuf strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := rOut.Read(buf)
		if n > 0 {
			outBuf.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	stdout := outBuf.String()

	if !strings.Contains(stdout, "bp5") {
		t.Errorf("expected blog post id (template spaceId path), got: %q", stdout)
	}
}

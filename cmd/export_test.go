// export_test.go exposes internal cmd package symbols for white-box testing.
// This file is compiled only during tests (package cmd_test import).
package cmd

import (
	"context"

	"github.com/sofq/confluence-cli/internal/client"
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

// SearchV1Domain exposes the package-private searchV1Domain helper for tests.
func SearchV1Domain(baseURL string) string {
	return searchV1Domain(baseURL)
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

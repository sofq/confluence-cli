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

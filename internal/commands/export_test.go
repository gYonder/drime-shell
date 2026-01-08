package commands

import (
	"context"

	"github.com/gYonder/drime-shell/internal/api"
)

// CheckCollisionsAndResolveWithPolicyForTest exposes checkCollisionsAndResolveWithPolicy for testing
func CheckCollisionsAndResolveWithPolicyForTest(ctx context.Context, client api.DrimeClient, workspaceID int64, parentID *int64, destPath string, sources []string, policy string) (map[string]string, error) {
	return checkCollisionsAndResolveWithPolicy(ctx, client, workspaceID, parentID, destPath, sources, policy)
}

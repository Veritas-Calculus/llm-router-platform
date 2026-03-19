// Package dataloaders provides batch-loading functions that prevent N+1 queries
// in GraphQL resolvers by collecting IDs within a single request and fetching
// them in one database round-trip.
package dataloaders

import (
	"context"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/user"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/graph-gophers/dataloader/v7"
)

// contextKey avoids collisions with other packages.
type contextKey string

const loadersKey contextKey = "dataloaders"

// Loaders holds all dataloaders for a single request.
type Loaders struct {
	APIKeysByUserID *dataloader.Loader[string, []models.APIKey]
}

// Middleware returns a Gin middleware that injects a fresh Loaders instance
// into every request's context. Each request gets its own dataloader
// instances so batching is scoped to a single request.
func Middleware(userSvc *user.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		loaders := &Loaders{
			APIKeysByUserID: dataloader.NewBatchedLoader(
				newAPIKeyBatchFn(userSvc),
				dataloader.WithWait[string, []models.APIKey](2*time.Millisecond),
			),
		}
		ctx := context.WithValue(c.Request.Context(), loadersKey, loaders)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// For returns the Loaders for the current request, or nil if not set.
func For(ctx context.Context) *Loaders {
	l, _ := ctx.Value(loadersKey).(*Loaders)
	return l
}

// ─── Batch functions ────────────────────────────────────────────────

func newAPIKeyBatchFn(userSvc *user.Service) dataloader.BatchFunc[string, []models.APIKey] {
	return func(ctx context.Context, userIDs []string) []*dataloader.Result[[]models.APIKey] {
		results := make([]*dataloader.Result[[]models.APIKey], len(userIDs))
		for i, uidStr := range userIDs {
			uid, err := uuid.Parse(uidStr)
			if err != nil {
				results[i] = &dataloader.Result[[]models.APIKey]{Error: err}
				continue
			}
			keys, err := userSvc.GetAPIKeys(ctx, uid)
			if err != nil {
				results[i] = &dataloader.Result[[]models.APIKey]{Error: err}
			} else {
				results[i] = &dataloader.Result[[]models.APIKey]{Data: keys}
			}
		}
		return results
	}
}

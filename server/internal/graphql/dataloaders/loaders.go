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

		// Parse all IDs up front; collect valid ones for the batch query and
		// pin Error results for any malformed string so callers see a
		// per-key error rather than a global failure.
		uuids := make([]uuid.UUID, 0, len(userIDs))
		idIndex := make(map[uuid.UUID][]int, len(userIDs)) // duplicates safe
		for i, uidStr := range userIDs {
			uid, err := uuid.Parse(uidStr)
			if err != nil {
				results[i] = &dataloader.Result[[]models.APIKey]{Error: err}
				continue
			}
			if _, ok := idIndex[uid]; !ok {
				uuids = append(uuids, uid)
			}
			idIndex[uid] = append(idIndex[uid], i)
		}

		if len(uuids) == 0 {
			return results
		}

		// One IN-query rather than len(uuids) sequential round-trips.
		grouped, err := userSvc.GetAPIKeysByUserIDs(ctx, uuids)
		if err != nil {
			// Fan the same error out to every position that didn't already
			// fail on UUID parse.
			for _, positions := range idIndex {
				for _, i := range positions {
					if results[i] == nil {
						results[i] = &dataloader.Result[[]models.APIKey]{Error: err}
					}
				}
			}
			return results
		}

		for uid, positions := range idIndex {
			keys := grouped[uid] // nil → empty slice in Data is fine
			for _, i := range positions {
				if results[i] == nil {
					results[i] = &dataloader.Result[[]models.APIKey]{Data: keys}
				}
			}
		}
		return results
	}
}

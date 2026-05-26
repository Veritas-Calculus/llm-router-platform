package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAPIKeyRepositoryGetByUserIDsGroupsResults(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE api_keys (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			project_id TEXT NOT NULL,
			channel TEXT,
			key_hash TEXT,
			key_prefix TEXT,
			name TEXT,
			is_active BOOLEAN,
			scopes TEXT,
			rate_limit INTEGER,
			token_limit INTEGER,
			daily_limit INTEGER,
			expires_at DATETIME,
			last_used_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)
	`).Error)

	userA := uuid.New()
	userB := uuid.New()
	userNoKeys := uuid.New()
	projectID := uuid.New()
	now := time.Now()

	insertKey := func(userID uuid.UUID, name string) {
		t.Helper()
		require.NoError(t, db.Exec(`
			INSERT INTO api_keys (
				id, user_id, project_id, channel, key_hash, key_prefix, name,
				is_active, scopes, rate_limit, token_limit, daily_limit,
				expires_at, last_used_at, created_at, updated_at
			) VALUES (?, ?, ?, 'default', ?, 'llm_test', ?, 1, 'all', 1000, 0, 10000, ?, ?, ?, ?)
		`, uuid.New().String(), userID.String(), projectID.String(), uuid.NewString(), name, now, now, now, now).Error)
	}

	insertKey(userA, "a-1")
	insertKey(userA, "a-2")
	insertKey(userB, "b-1")

	grouped, err := NewAPIKeyRepository(db).GetByUserIDs(context.Background(), []uuid.UUID{userA, userB, userNoKeys})
	require.NoError(t, err)

	require.Len(t, grouped[userA], 2)
	require.Len(t, grouped[userB], 1)
	require.Empty(t, grouped[userNoKeys])
	require.Equal(t, "b-1", grouped[userB][0].Name)
}

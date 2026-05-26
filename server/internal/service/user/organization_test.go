package user

import (
	"context"
	"testing"

	"llm-router-platform/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newOrgRoleTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT, password_hash TEXT, role TEXT)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE organization_members (org_id TEXT, user_id TEXT, role TEXT, PRIMARY KEY (org_id, user_id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE projects (id TEXT PRIMARY KEY, org_id TEXT NOT NULL, name TEXT)`).Error)

	return &Service{orgRepo: repository.NewOrganizationRepository(db), logger: zap.NewNop()}, db
}

func TestRequireOrgRole_PlatformAdminBypassesMembership(t *testing.T) {
	svc, db := newOrgRoleTestService(t)

	adminID := uuid.New()
	ownerID := uuid.New()
	orgID := uuid.New()
	projectID := uuid.New()

	require.NoError(t, db.Exec(`INSERT INTO users (id, email, password_hash, role) VALUES (?, ?, ?, ?)`, adminID.String(), "admin@example.com", "x", "admin").Error)
	require.NoError(t, db.Exec(`INSERT INTO users (id, email, password_hash, role) VALUES (?, ?, ?, ?)`, ownerID.String(), "owner@example.com", "x", "user").Error)
	require.NoError(t, db.Exec(`INSERT INTO projects (id, org_id, name) VALUES (?, ?, ?)`, projectID.String(), orgID.String(), "Project").Error)

	require.NoError(t, svc.RequireOrgRole(context.Background(), adminID.String(), orgID.String(), "OWNER"))
	require.NoError(t, svc.RequireProjectRole(context.Background(), adminID.String(), projectID.String(), "OWNER"))
}

func TestRequireOrgRole_AllowedRolesAreCaseInsensitive(t *testing.T) {
	svc, db := newOrgRoleTestService(t)

	userID := uuid.New()
	orgID := uuid.New()

	require.NoError(t, db.Exec(`INSERT INTO users (id, email, password_hash, role) VALUES (?, ?, ?, ?)`, userID.String(), "member@example.com", "x", "user").Error)
	require.NoError(t, db.Exec(`INSERT INTO organization_members (org_id, user_id, role) VALUES (?, ?, ?)`, orgID.String(), userID.String(), "ADMIN").Error)

	require.NoError(t, svc.RequireOrgRole(context.Background(), userID.String(), orgID.String(), "admin"))
	require.NoError(t, svc.RequireOrgRole(context.Background(), userID.String(), orgID.String(), "Admin"))
}

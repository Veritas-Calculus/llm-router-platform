package resolvers

import (
	"context"
	"testing"

	"llm-router-platform/internal/graphql/model"
	"llm-router-platform/internal/service/settings"

	"go.uber.org/zap"
)

// fakeSettingsReader returns whatever the test plugs in. Concrete impl
// of settings.settingsReader (an unexported interface), reached here via
// the exported settings.New constructor.
type fakeSettingsReader struct {
	data map[string]string
}

func (f *fakeSettingsReader) GetAllSettingsDecrypted(_ context.Context) (map[string]string, error) {
	out := make(map[string]string, len(f.data))
	for k, v := range f.data {
		out[k] = v
	}
	return out, nil
}

// TestRegistrationModeResolver_FlipsWithRegistry asserts that the public
// registrationMode query reflects whatever the DB-backed
// settings.Registry currently exposes — proving the Register resolver
// path will obey too, since both read from the same source.
func TestRegistrationModeResolver_FlipsWithRegistry(t *testing.T) {
	reader := &fakeSettingsReader{data: map[string]string{
		"security": `{"registrationMode":"open"}`,
	}}
	reg := settings.New(reader, settings.BuiltinDefaults(), zap.NewNop())
	r := &Resolver{SettingsRegistry: reg}
	q := &queryResolver{Resolver: r}

	got, err := q.RegistrationMode(context.Background())
	if err != nil {
		t.Fatalf("first call: unexpected error %v", err)
	}
	if got.Mode != "open" {
		t.Fatalf("expected open, got %q", got.Mode)
	}

	// Flip the DB row and invalidate the cache to simulate the
	// updateSystemSettings → Invalidate path.
	reader.data["security"] = `{"registrationMode":"closed"}`
	reg.Invalidate(context.Background(), "security")

	got, err = q.RegistrationMode(context.Background())
	if err != nil {
		t.Fatalf("second call: unexpected error %v", err)
	}
	if got.Mode != "closed" {
		t.Fatalf("expected closed after flip, got %q", got.Mode)
	}
}

// TestRegistrationModeResolver_NilRegistryFallsBackToClosed pins the
// behaviour when the SettingsRegistry isn't wired (e.g. tests built
// directly): we fail-safe to "closed" so registration is denied rather
// than accidentally opened.
func TestRegistrationModeResolver_NilRegistryFallsBackToClosed(t *testing.T) {
	r := &Resolver{}
	q := &queryResolver{Resolver: r}

	got, err := q.RegistrationMode(context.Background())
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if got.Mode != "closed" {
		t.Fatalf("expected closed fallback, got %q", got.Mode)
	}
	// Ensure the shape returned is well-formed enough for the SPA.
	if got.InviteCodeRequired {
		t.Error("InviteCodeRequired should be false for closed mode")
	}
}

// _ keeps the model import live in case future test additions want it.
var _ = model.RegistrationMode{}

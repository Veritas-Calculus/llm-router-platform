package provider

import (
	"go.uber.org/zap"
)

// Registry holds all registered provider clients.
type Registry struct {
	clients map[string]Client
	logger  *zap.Logger
}

// NewRegistry creates a new provider registry.
func NewRegistry(logger *zap.Logger) *Registry {
	return &Registry{
		clients: make(map[string]Client),
		logger:  logger,
	}
}

// Register adds a provider client to the registry.
func (r *Registry) Register(name string, client Client) {
	r.clients[name] = client
}

// Get retrieves a provider client by name.
func (r *Registry) Get(name string) (Client, bool) {
	client, ok := r.clients[name]
	return client, ok
}

// List returns all registered provider names.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.clients))
	for name := range r.clients {
		names = append(names, name)
	}
	return names
}

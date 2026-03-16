package provider

import (
	"go.uber.org/zap"
)

// Capability represents a feature that a provider supports.
type Capability string

const (
	CapChat       Capability = "chat"
	CapStream     Capability = "stream"
	CapEmbeddings Capability = "embeddings"
	CapImage      Capability = "image"
	CapAudio      Capability = "audio"
	CapTTS        Capability = "tts"
	CapVideo      Capability = "video"
)

// ProviderInfo holds a client and its declared capabilities.
type ProviderInfo struct {
	Client       Client
	Capabilities map[Capability]bool
}

// Registry holds all registered provider clients and their capabilities.
type Registry struct {
	providers map[string]*ProviderInfo
	logger    *zap.Logger
}

// NewRegistry creates a new provider registry.
func NewRegistry(logger *zap.Logger) *Registry {
	return &Registry{
		providers: make(map[string]*ProviderInfo),
		logger:    logger,
	}
}

// Register adds a provider client to the registry with default capabilities.
func (r *Registry) Register(name string, client Client) {
	r.providers[name] = &ProviderInfo{
		Client: client,
		Capabilities: map[Capability]bool{
			CapChat:   true,
			CapStream: true,
		},
	}
}

// RegisterWithCapabilities adds a provider client with explicit capabilities.
func (r *Registry) RegisterWithCapabilities(name string, client Client, caps ...Capability) {
	capMap := make(map[Capability]bool, len(caps))
	for _, c := range caps {
		capMap[c] = true
	}
	r.providers[name] = &ProviderInfo{
		Client:       client,
		Capabilities: capMap,
	}
}

// Get retrieves a provider client by name.
func (r *Registry) Get(name string) (Client, bool) {
	info, ok := r.providers[name]
	if !ok {
		return nil, false
	}
	return info.Client, true
}

// GetInfo retrieves full provider info (client + capabilities).
func (r *Registry) GetInfo(name string) (*ProviderInfo, bool) {
	info, ok := r.providers[name]
	return info, ok
}

// HasCapability checks if a provider supports a specific capability.
func (r *Registry) HasCapability(name string, cap Capability) bool {
	info, ok := r.providers[name]
	if !ok {
		return false
	}
	return info.Capabilities[cap]
}

// List returns all registered provider names.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// CapabilityMatrix returns a map of provider → supported capabilities.
func (r *Registry) CapabilityMatrix() map[string][]Capability {
	matrix := make(map[string][]Capability, len(r.providers))
	for name, info := range r.providers {
		caps := make([]Capability, 0)
		for cap, supported := range info.Capabilities {
			if supported {
				caps = append(caps, cap)
			}
		}
		matrix[name] = caps
	}
	return matrix
}

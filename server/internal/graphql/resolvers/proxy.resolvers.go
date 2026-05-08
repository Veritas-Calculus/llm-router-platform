package resolvers

// This file contains proxy domain resolvers.
// Extracted from schema.resolvers.go for maintainability.

import (
	"context"
	"llm-router-platform/internal/graphql/model"

	"github.com/google/uuid"
)

// CreateProxy is the resolver for the createProxy field.
func (r *mutationResolver) CreateProxy(ctx context.Context, input model.ProxyInput) (*model.Proxy, error) {
	var upstreamID *uuid.UUID
	if input.UpstreamProxyID != nil && *input.UpstreamProxyID != "" {
		id, _ := uuid.Parse(*input.UpstreamProxyID)
		upstreamID = &id
	}
	poolID := parseOptionalUUID(input.PoolID)
	p, err := r.Proxy.Create(ctx, input.URL, input.Type, derefStr(input.Region), derefStr(input.Username), derefStr(input.Password), upstreamID, poolID)
	if err != nil {
		return nil, err
	}
	return proxyToGQL(p), nil
}

// CreateProxyPool is the resolver for the createProxyPool field.
func (r *mutationResolver) CreateProxyPool(ctx context.Context, input model.ProxyPoolInput) (*model.ProxyPool, error) {
	pool, err := r.Proxy.CreatePool(ctx, input.Name, derefStr(input.Description), derefStrDefault(input.Strategy, "weighted"))
	if err != nil {
		return nil, err
	}
	if input.IsActive != nil && pool.IsActive != *input.IsActive {
		pool.IsActive = *input.IsActive
		if err := r.AdminSvc.DB().Save(pool).Error; err != nil {
			return nil, err
		}
	}
	return proxyPoolToGQL(pool), nil
}

// UpdateProxyPool is the resolver for the updateProxyPool field.
func (r *mutationResolver) UpdateProxyPool(ctx context.Context, id string, input model.ProxyPoolInput) (*model.ProxyPool, error) {
	pid, _ := uuid.Parse(id)
	pool, err := r.Proxy.UpdatePool(ctx, pid, input.Name, derefStr(input.Description), derefStrDefault(input.Strategy, "weighted"), derefBool(input.IsActive, true))
	if err != nil {
		return nil, err
	}
	return proxyPoolToGQL(pool), nil
}

// DeleteProxyPool is the resolver for the deleteProxyPool field.
func (r *mutationResolver) DeleteProxyPool(ctx context.Context, id string) (bool, error) {
	pid, _ := uuid.Parse(id)
	return true, r.Proxy.DeletePool(ctx, pid)
}

// BatchCreateProxies is the resolver for the batchCreateProxies field.
func (r *mutationResolver) BatchCreateProxies(ctx context.Context, input model.BatchProxyInput) (*model.BatchProxyResult, error) {
	result := &model.BatchProxyResult{Proxies: []*model.Proxy{}}
	poolID := parseOptionalUUID(input.PoolID)
	for _, item := range input.Proxies {
		typ := "http"
		if item.Type != nil {
			typ = *item.Type
		}
		p, err := r.Proxy.Create(ctx, item.URL, typ, derefStr(item.Region), "", "", nil, poolID)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, item.URL+": "+err.Error())
		} else {
			result.Success++
			result.Proxies = append(result.Proxies, proxyToGQL(p))
		}
	}
	return result, nil
}

// UpdateProxy is the resolver for the updateProxy field.
func (r *mutationResolver) UpdateProxy(ctx context.Context, id string, input model.ProxyInput) (*model.Proxy, error) {
	pid, _ := uuid.Parse(id)
	var upstreamID *uuid.UUID
	if input.UpstreamProxyID != nil && *input.UpstreamProxyID != "" {
		uid, _ := uuid.Parse(*input.UpstreamProxyID)
		upstreamID = &uid
	}
	poolID := parseOptionalUUID(input.PoolID)
	p, err := r.Proxy.Update(ctx, pid, input.URL, input.Type, derefStr(input.Region), true, derefStr(input.Username), derefStr(input.Password), upstreamID, poolID)
	if err != nil {
		return nil, err
	}
	return proxyToGQL(p), nil
}

// DeleteProxy is the resolver for the deleteProxy field.
func (r *mutationResolver) DeleteProxy(ctx context.Context, id string) (bool, error) {
	pid, _ := uuid.Parse(id)
	return true, r.Proxy.Delete(ctx, pid)
}

// ToggleProxyStatus is the resolver for the toggleProxyStatus field.
func (r *mutationResolver) ToggleProxyStatus(ctx context.Context, id string) (*model.Proxy, error) {
	pid, _ := uuid.Parse(id)
	p, err := r.Proxy.Toggle(ctx, pid)
	if err != nil {
		return nil, err
	}
	return proxyToGQL(p), nil
}

// TestProxy is the resolver for the testProxy field.
func (r *mutationResolver) TestProxy(ctx context.Context, id string) (*model.ProxyTestResult, error) {
	pid, _ := uuid.Parse(id)
	healthy, latency, testErr := r.Proxy.CheckHealth(ctx, pid)
	p, err := r.Proxy.GetByID(ctx, pid)
	if err != nil {
		return nil, err
	}
	result := &model.ProxyTestResult{ID: p.ID.String(), URL: p.URL, IsHealthy: healthy, LatencyMs: float64(latency.Milliseconds())}
	if testErr != nil {
		e := testErr.Error()
		result.Error = &e
	}
	return result, nil
}

// TestAllProxies is the resolver for the testAllProxies field.
func (r *mutationResolver) TestAllProxies(ctx context.Context) ([]*model.ProxyTestResult, error) {
	proxies, err := r.Proxy.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*model.ProxyTestResult, 0, len(proxies))
	for _, p := range proxies {
		healthy, latency, testErr := r.Proxy.CheckHealth(ctx, p.ID)
		result := &model.ProxyTestResult{ID: p.ID.String(), URL: p.URL, IsHealthy: healthy, LatencyMs: float64(latency.Milliseconds())}
		if testErr != nil {
			e := testErr.Error()
			result.Error = &e
		}
		out = append(out, result)
	}
	return out, nil
}

// Proxies is the resolver for the proxies field.
func (r *queryResolver) Proxies(ctx context.Context) ([]*model.Proxy, error) {
	proxies, err := r.Proxy.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*model.Proxy, len(proxies))
	for i := range proxies {
		out[i] = proxyToGQL(&proxies[i])
	}
	return out, nil
}

// ProxyPools is the resolver for the proxyPools field.
func (r *queryResolver) ProxyPools(ctx context.Context) ([]*model.ProxyPool, error) {
	pools, err := r.Proxy.GetPools(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*model.ProxyPool, len(pools))
	for i := range pools {
		out[i] = proxyPoolToGQL(&pools[i])
	}
	return out, nil
}

func parseOptionalUUID(value *string) *uuid.UUID {
	if value == nil || *value == "" {
		return nil
	}
	id, err := uuid.Parse(*value)
	if err != nil {
		return nil
	}
	return &id
}

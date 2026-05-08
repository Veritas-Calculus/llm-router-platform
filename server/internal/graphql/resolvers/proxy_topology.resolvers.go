package resolvers

import (
	"context"
	"fmt"

	"llm-router-platform/internal/graphql/model"
	"llm-router-platform/internal/models"

	"github.com/google/uuid"
)

// ProxyTopology is the resolver for the proxyTopology field.
func (r *queryResolver) ProxyTopology(ctx context.Context) (*model.ProxyTopology, error) {
	providers, err := r.Router.GetAllProviders(ctx)
	if err != nil {
		return nil, err
	}
	proxies, err := r.Proxy.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	pools, err := r.Proxy.GetPools(ctx)
	if err != nil {
		return nil, err
	}

	proxyByID := make(map[uuid.UUID]models.Proxy, len(proxies))
	activeProxies := make([]models.Proxy, 0, len(proxies))
	for i := range proxies {
		proxyByID[proxies[i].ID] = proxies[i]
		if proxies[i].IsActive {
			activeProxies = append(activeProxies, proxies[i])
		}
	}

	poolByID := make(map[uuid.UUID]models.ProxyPool, len(pools))
	activeProxiesByPool := make(map[uuid.UUID][]models.Proxy)
	poolNodes := make([]*model.ProxyPoolTopologyNode, 0, len(pools))
	for i := range pools {
		poolByID[pools[i].ID] = pools[i]
		poolProxies := make([]*model.Proxy, 0, len(pools[i].Proxies))
		for j := range pools[i].Proxies {
			p := pools[i].Proxies[j]
			poolProxies = append(poolProxies, proxyToGQL(&p))
			if p.IsActive {
				activeProxiesByPool[pools[i].ID] = append(activeProxiesByPool[pools[i].ID], p)
			}
		}
		poolNodes = append(poolNodes, &model.ProxyPoolTopologyNode{
			Pool:    proxyPoolToGQL(&pools[i]),
			Proxies: poolProxies,
		})
	}

	topology := &model.ProxyTopology{
		Providers:  make([]*model.ProviderTopologyNode, 0, len(providers)),
		ProxyPools: poolNodes,
	}

	for i := range providers {
		p := &providers[i]
		keys, err := r.Router.GetAllProviderAPIKeys(ctx, p.ID)
		if err != nil {
			return nil, err
		}

		node := &model.ProviderTopologyNode{
			Provider: providerToGQL(p),
			Models:   providerModelNames(p.Models),
			Accounts: make([]*model.ProviderAccountTopologyNode, 0, len(keys)),
		}

		if len(keys) == 0 && !p.RequiresAPIKey {
			account := r.providerAccountTopology(p, nil, proxyByID, poolByID, activeProxiesByPool, activeProxies)
			node.Accounts = append(node.Accounts, account.ProviderAccountTopologyNode)
			if account.bindingSource == "direct" {
				topology.DirectAccounts++
			} else {
				topology.ProxiedAccounts++
			}
		}

		for j := range keys {
			account := r.providerAccountTopology(p, &keys[j], proxyByID, poolByID, activeProxiesByPool, activeProxies)
			node.Accounts = append(node.Accounts, account.ProviderAccountTopologyNode)
			if account.bindingSource == "direct" {
				topology.DirectAccounts++
			} else {
				topology.ProxiedAccounts++
			}
		}
		topology.Providers = append(topology.Providers, node)
	}

	return topology, nil
}

func providerModelNames(providerModels []models.Model) []string {
	names := make([]string, 0, len(providerModels))
	for i := range providerModels {
		if providerModels[i].IsActive {
			names = append(names, providerModels[i].Name)
		}
	}
	return names
}

type providerAccountTopologyNode struct {
	*model.ProviderAccountTopologyNode
	bindingSource string
}

func (r *queryResolver) providerAccountTopology(
	p *models.Provider,
	key *models.ProviderAPIKey,
	proxyByID map[uuid.UUID]models.Proxy,
	poolByID map[uuid.UUID]models.ProxyPool,
	activeProxiesByPool map[uuid.UUID][]models.Proxy,
	activeProxies []models.Proxy,
) *providerAccountTopologyNode {
	label := "No API key required"
	var keyModel *model.ProviderAPIKey
	if key != nil {
		keyModel = providerAPIKeyToGQL(key)
		if key.Alias != "" {
			label = key.Alias
		} else {
			label = key.KeyPrefix
		}
	}

	source := "direct"
	route := []*model.ProxyTopologyRouteStep{topologyStep(accountStepID(p, key), "account", label, "active", nil)}
	candidates := []*model.Proxy{}
	var poolModel *model.ProxyPool

	if key != nil && key.ProxyID != nil {
		source = "api_key_proxy"
		if proxyInfo, ok := proxyByID[*key.ProxyID]; ok {
			route = append(route, proxyChainSteps(&proxyInfo, proxyByID)...)
		} else {
			route = append(route, topologyStep(key.ProxyID.String(), "proxy", "Missing bound proxy", "missing", nil))
		}
	} else if key != nil && key.ProxyPoolID != nil {
		source = "api_key_proxy_pool"
		if pool, ok := poolByID[*key.ProxyPoolID]; ok {
			poolModel = proxyPoolToGQL(&pool)
			route = append(route, topologyStep(pool.ID.String(), "proxy_pool", pool.Name, topologyStatus(pool.IsActive), strPtr(pool.Strategy)))
			candidates = proxiesToGQL(activeProxiesByPool[pool.ID])
		} else {
			route = append(route, topologyStep(key.ProxyPoolID.String(), "proxy_pool", "Missing bound proxy pool", "missing", nil))
		}
	} else if p.UseProxy && p.DefaultProxyID != nil {
		source = "provider_default_proxy"
		if proxyInfo, ok := proxyByID[*p.DefaultProxyID]; ok {
			route = append(route, proxyChainSteps(&proxyInfo, proxyByID)...)
		} else {
			route = append(route, topologyStep(p.DefaultProxyID.String(), "proxy", "Missing provider default proxy", "missing", nil))
		}
	} else if p.UseProxy {
		source = "provider_proxy_pool"
		candidates = proxiesToGQL(activeProxies)
		route = append(route, topologyStep("active-proxies", "proxy_pool", "Any active proxy", "active", strPtr(fmt.Sprintf("%d candidates", len(activeProxies)))))
	}

	route = append(route, topologyStep(p.ID.String(), "provider", p.Name, topologyStatus(p.IsActive), strPtr(p.BaseURL)))

	return &providerAccountTopologyNode{
		ProviderAccountTopologyNode: &model.ProviderAccountTopologyNode{
			APIKey:           keyModel,
			Label:            label,
			BindingSource:    source,
			ProxyPool:        poolModel,
			CandidateProxies: candidates,
			Route:            route,
		},
		bindingSource: source,
	}
}

func accountStepID(p *models.Provider, key *models.ProviderAPIKey) string {
	if key == nil {
		return p.ID.String() + ":keyless"
	}
	return key.ID.String()
}

func proxyChainSteps(proxyInfo *models.Proxy, proxyByID map[uuid.UUID]models.Proxy) []*model.ProxyTopologyRouteStep {
	return proxyChainStepsVisited(proxyInfo, proxyByID, map[uuid.UUID]bool{}, 0)
}

func proxyChainStepsVisited(proxyInfo *models.Proxy, proxyByID map[uuid.UUID]models.Proxy, visited map[uuid.UUID]bool, depth int) []*model.ProxyTopologyRouteStep {
	if proxyInfo == nil || depth > 8 {
		return nil
	}
	if visited[proxyInfo.ID] {
		return []*model.ProxyTopologyRouteStep{topologyStep(proxyInfo.ID.String(), "proxy", proxyInfo.URL, "cycle", strPtr("proxy chain cycle detected"))}
	}
	visited[proxyInfo.ID] = true

	steps := []*model.ProxyTopologyRouteStep{}
	if proxyInfo.UpstreamProxyID != nil {
		if upstream, ok := proxyByID[*proxyInfo.UpstreamProxyID]; ok {
			steps = append(steps, proxyChainStepsVisited(&upstream, proxyByID, visited, depth+1)...)
		} else {
			steps = append(steps, topologyStep(proxyInfo.UpstreamProxyID.String(), "proxy", "Missing upstream proxy", "missing", nil))
		}
	}
	steps = append(steps, topologyStep(proxyInfo.ID.String(), "proxy", proxyInfo.URL, topologyStatus(proxyInfo.IsActive), strPtr(proxyInfo.Type)))
	return steps
}

func proxiesToGQL(proxies []models.Proxy) []*model.Proxy {
	out := make([]*model.Proxy, 0, len(proxies))
	for i := range proxies {
		out = append(out, proxyToGQL(&proxies[i]))
	}
	return out
}

func topologyStep(id, typ, label, status string, detail *string) *model.ProxyTopologyRouteStep {
	return &model.ProxyTopologyRouteStep{ID: id, Type: typ, Label: label, Status: status, Detail: detail}
}

func topologyStatus(active bool) string {
	if active {
		return "active"
	}
	return "inactive"
}

func strPtr(v string) *string {
	return &v
}

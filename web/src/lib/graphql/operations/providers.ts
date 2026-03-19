import { gql } from '@apollo/client';

// ── Provider Operations ─────────────────────────────────────────────

export const PROVIDERS_QUERY = gql`
  query Providers {
    providers {
      id name baseUrl isActive priority weight maxRetries timeout useProxy requiresApiKey createdAt
    }
  }
`;

export const PROVIDER_API_KEYS_QUERY = gql`
  query ProviderApiKeys($providerId: ID!) {
    providerApiKeys(providerId: $providerId) {
      id providerId alias keyPrefix isActive priority weight rateLimit usageCount lastUsedAt createdAt
    }
  }
`;

export const PROVIDER_HEALTH_QUERY = gql`
  query ProviderHealthStatus($providerId: ID!) {
    providerHealth(providerId: $providerId) {
      providerId providerName isHealthy latency lastChecked activeKeyCount totalKeyCount
    }
  }
`;

export const UPDATE_PROVIDER = gql`
  mutation UpdateProvider($id: ID!, $input: ProviderInput!) {
    updateProvider(id: $id, input: $input) {
      id name baseUrl isActive priority weight maxRetries timeout useProxy
    }
  }
`;

export const TOGGLE_PROVIDER = gql`
  mutation ToggleProvider($id: ID!) {
    toggleProvider(id: $id) { id isActive }
  }
`;

export const TOGGLE_PROVIDER_PROXY = gql`
  mutation ToggleProviderProxy($id: ID!) {
    toggleProviderProxy(id: $id) { id useProxy }
  }
`;

export const CREATE_PROVIDER_API_KEY = gql`
  mutation CreateProviderApiKey($providerId: ID!, $input: ProviderApiKeyInput!) {
    createProviderApiKey(providerId: $providerId, input: $input) {
      id providerId alias keyPrefix isActive priority weight rateLimit usageCount createdAt
    }
  }
`;

export const UPDATE_PROVIDER_API_KEY = gql`
  mutation UpdateProviderApiKey($providerId: ID!, $keyId: ID!, $input: UpdateProviderApiKeyInput!) {
    updateProviderApiKey(providerId: $providerId, keyId: $keyId, input: $input) {
      id isActive priority weight
    }
  }
`;

export const TOGGLE_PROVIDER_API_KEY = gql`
  mutation ToggleProviderApiKey($providerId: ID!, $keyId: ID!) {
    toggleProviderApiKey(providerId: $providerId, keyId: $keyId) { id isActive }
  }
`;

export const DELETE_PROVIDER_API_KEY = gql`
  mutation DeleteProviderApiKey($providerId: ID!, $keyId: ID!) {
    deleteProviderApiKey(providerId: $providerId, keyId: $keyId)
  }
`;

// ── Model Operations ──────────────────────────────────────────────

export const MODELS_QUERY = gql`
  query Models($providerId: ID!) {
    models(providerId: $providerId) {
      id providerId name displayName inputPricePer1k outputPricePer1k
      pricePerSecond pricePerImage pricePerMinute maxTokens isActive createdAt
    }
  }
`;

export const CREATE_MODEL = gql`
  mutation CreateModel($providerId: ID!, $input: ModelInput!) {
    createModel(providerId: $providerId, input: $input) {
      id name displayName inputPricePer1k outputPricePer1k maxTokens isActive
    }
  }
`;

export const UPDATE_MODEL = gql`
  mutation UpdateModel($id: ID!, $input: ModelInput!) {
    updateModel(id: $id, input: $input) {
      id name displayName inputPricePer1k outputPricePer1k maxTokens isActive
    }
  }
`;

export const DELETE_MODEL = gql`
  mutation DeleteModel($id: ID!) {
    deleteModel(id: $id)
  }
`;

export const TOGGLE_MODEL = gql`
  mutation ToggleModel($id: ID!) {
    toggleModel(id: $id) { id isActive }
  }
`;

export const SYNC_PROVIDER_MODELS = gql`
  mutation SyncProviderModels($providerId: ID!) {
    syncProviderModels(providerId: $providerId) {
      id providerId name displayName inputPricePer1k outputPricePer1k maxTokens isActive
    }
  }
`;

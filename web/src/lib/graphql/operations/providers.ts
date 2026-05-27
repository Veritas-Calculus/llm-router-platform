import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  CreateModelMutation,
  CreateModelMutationVariables,
  CreateProviderApiKeyMutation,
  CreateProviderApiKeyMutationVariables,
  CreateProviderMutation,
  CreateProviderMutationVariables,
  DeleteModelMutation,
  DeleteModelMutationVariables,
  DeleteProviderApiKeyMutation,
  DeleteProviderApiKeyMutationVariables,
  DeleteProviderMutation,
  DeleteProviderMutationVariables,
  ModelsQuery,
  ModelsQueryVariables,
  ProviderApiKeysQuery,
  ProviderApiKeysQueryVariables,
  ProviderHealthStatusQuery,
  ProviderHealthStatusQueryVariables,
  ProvidersQuery,
  ProvidersQueryVariables,
  SyncProviderModelsMutation,
  SyncProviderModelsMutationVariables,
  ToggleModelMutation,
  ToggleModelMutationVariables,
  ToggleProviderApiKeyMutation,
  ToggleProviderApiKeyMutationVariables,
  ToggleProviderMutation,
  ToggleProviderMutationVariables,
  ToggleProviderProxyMutation,
  ToggleProviderProxyMutationVariables,
  UpdateModelMutation,
  UpdateModelMutationVariables,
  UpdateProviderApiKeyMutation,
  UpdateProviderApiKeyMutationVariables,
  UpdateProviderMutation,
  UpdateProviderMutationVariables,
} from '../generated/graphql';

// ── Provider Operations ─────────────────────────────────────────────

export const PROVIDERS_QUERY: TypedDocumentNode<ProvidersQuery, ProvidersQueryVariables> = gql`
  query Providers {
    providers {
      id name baseUrl isActive priority weight maxRetries timeout useProxy defaultProxyId requiresApiKey createdAt
    }
  }
`;

export const CREATE_PROVIDER: TypedDocumentNode<CreateProviderMutation, CreateProviderMutationVariables> = gql`
  mutation CreateProvider($input: CreateProviderInput!) {
    createProvider(input: $input) {
      id name baseUrl isActive priority weight maxRetries timeout useProxy defaultProxyId requiresApiKey createdAt
    }
  }
`;

export const DELETE_PROVIDER: TypedDocumentNode<DeleteProviderMutation, DeleteProviderMutationVariables> = gql`
  mutation DeleteProvider($id: ID!) {
    deleteProvider(id: $id)
  }
`;

export const PROVIDER_API_KEYS_QUERY: TypedDocumentNode<ProviderApiKeysQuery, ProviderApiKeysQueryVariables> = gql`
  query ProviderApiKeys($providerId: ID!) {
    providerApiKeys(providerId: $providerId) {
      id providerId proxyId proxyPoolId alias keyPrefix isActive priority weight rateLimit usageCount lastUsedAt createdAt
    }
  }
`;

export const PROVIDER_HEALTH_QUERY: TypedDocumentNode<ProviderHealthStatusQuery, ProviderHealthStatusQueryVariables> = gql`
  query ProviderHealthStatus($providerId: ID!) {
    providerHealth(providerId: $providerId) {
      id name baseUrl isActive isHealthy useProxy responseTime lastCheck successRate errorMessage
    }
  }
`;

export const UPDATE_PROVIDER: TypedDocumentNode<UpdateProviderMutation, UpdateProviderMutationVariables> = gql`
  mutation UpdateProvider($id: ID!, $input: ProviderInput!) {
    updateProvider(id: $id, input: $input) {
      id name baseUrl isActive priority weight maxRetries timeout useProxy defaultProxyId requiresApiKey createdAt
    }
  }
`;

export const TOGGLE_PROVIDER: TypedDocumentNode<ToggleProviderMutation, ToggleProviderMutationVariables> = gql`
  mutation ToggleProvider($id: ID!) {
    toggleProvider(id: $id) { id isActive }
  }
`;

export const TOGGLE_PROVIDER_PROXY: TypedDocumentNode<ToggleProviderProxyMutation, ToggleProviderProxyMutationVariables> = gql`
  mutation ToggleProviderProxy($id: ID!) {
    toggleProviderProxy(id: $id) { id useProxy }
  }
`;

export const CREATE_PROVIDER_API_KEY: TypedDocumentNode<CreateProviderApiKeyMutation, CreateProviderApiKeyMutationVariables> = gql`
  mutation CreateProviderApiKey($providerId: ID!, $input: ProviderApiKeyInput!) {
    createProviderApiKey(providerId: $providerId, input: $input) {
      id providerId proxyId proxyPoolId alias keyPrefix isActive priority weight rateLimit usageCount createdAt
    }
  }
`;

export const UPDATE_PROVIDER_API_KEY: TypedDocumentNode<UpdateProviderApiKeyMutation, UpdateProviderApiKeyMutationVariables> = gql`
  mutation UpdateProviderApiKey($providerId: ID!, $keyId: ID!, $input: UpdateProviderApiKeyInput!) {
    updateProviderApiKey(providerId: $providerId, keyId: $keyId, input: $input) {
      id proxyId proxyPoolId isActive priority weight rateLimit
    }
  }
`;

export const TOGGLE_PROVIDER_API_KEY: TypedDocumentNode<ToggleProviderApiKeyMutation, ToggleProviderApiKeyMutationVariables> = gql`
  mutation ToggleProviderApiKey($providerId: ID!, $keyId: ID!) {
    toggleProviderApiKey(providerId: $providerId, keyId: $keyId) { id isActive }
  }
`;

export const DELETE_PROVIDER_API_KEY: TypedDocumentNode<DeleteProviderApiKeyMutation, DeleteProviderApiKeyMutationVariables> = gql`
  mutation DeleteProviderApiKey($providerId: ID!, $keyId: ID!) {
    deleteProviderApiKey(providerId: $providerId, keyId: $keyId)
  }
`;

// ── Model Operations ──────────────────────────────────────────────

export const MODELS_QUERY: TypedDocumentNode<ModelsQuery, ModelsQueryVariables> = gql`
  query Models($providerId: ID!) {
    models(providerId: $providerId) {
      id providerId name displayName modelKind
      inputPricePer1k outputPricePer1k
      pricePerSecond pricePerImage pricePerMinute
      providerInputCostPer1k providerOutputCostPer1k
      providerCostPerSecond providerCostPerImage providerCostPerMinute
      contextWindow maxOutputTokens catalogWarnings
      isActive createdAt
    }
  }
`;

export const CREATE_MODEL: TypedDocumentNode<CreateModelMutation, CreateModelMutationVariables> = gql`
  mutation CreateModel($providerId: ID!, $input: ModelInput!) {
    createModel(providerId: $providerId, input: $input) {
      id name displayName modelKind inputPricePer1k outputPricePer1k
      providerInputCostPer1k providerOutputCostPer1k
      contextWindow maxOutputTokens isActive
    }
  }
`;

export const UPDATE_MODEL: TypedDocumentNode<UpdateModelMutation, UpdateModelMutationVariables> = gql`
  mutation UpdateModel($id: ID!, $input: ModelInput!) {
    updateModel(id: $id, input: $input) {
      id name displayName modelKind inputPricePer1k outputPricePer1k
      providerInputCostPer1k providerOutputCostPer1k
      contextWindow maxOutputTokens isActive
    }
  }
`;

export const DELETE_MODEL: TypedDocumentNode<DeleteModelMutation, DeleteModelMutationVariables> = gql`
  mutation DeleteModel($id: ID!) {
    deleteModel(id: $id)
  }
`;

export const TOGGLE_MODEL: TypedDocumentNode<ToggleModelMutation, ToggleModelMutationVariables> = gql`
  mutation ToggleModel($id: ID!) {
    toggleModel(id: $id) { id isActive }
  }
`;

export const SYNC_PROVIDER_MODELS: TypedDocumentNode<SyncProviderModelsMutation, SyncProviderModelsMutationVariables> = gql`
  mutation SyncProviderModels($providerId: ID!) {
    syncProviderModels(providerId: $providerId) {
      id providerId name displayName modelKind
      inputPricePer1k outputPricePer1k
      contextWindow maxOutputTokens isActive
    }
  }
`;

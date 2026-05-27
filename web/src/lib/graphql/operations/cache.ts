import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  CacheConfigQuery,
  CacheConfigQueryVariables,
  ClearAllSemanticCachesMutation,
  ClearAllSemanticCachesMutationVariables,
  ClearSemanticCacheMutation,
  ClearSemanticCacheMutationVariables,
  GetCacheStatsQuery,
  GetCacheStatsQueryVariables,
  GetSemanticCachesQuery,
  GetSemanticCachesQueryVariables,
  UpdateCacheConfigMutation,
  UpdateCacheConfigMutationVariables,
} from '../generated/graphql';

export const GET_SEMANTIC_CACHES: TypedDocumentNode<GetSemanticCachesQuery, GetSemanticCachesQueryVariables> = gql`
  query GetSemanticCaches($limit: Int, $offset: Int) {
    semanticCaches(limit: $limit, offset: $offset) {
      id
      hash
      provider
      model
      hitCount
      createdAt
    }
  }
`;

export const GET_CACHE_STATS: TypedDocumentNode<GetCacheStatsQuery, GetCacheStatsQueryVariables> = gql`
  query GetCacheStats {
    cacheStats {
      totalCaches
      totalHits
    }
  }
`;

export const CLEAR_SEMANTIC_CACHE: TypedDocumentNode<ClearSemanticCacheMutation, ClearSemanticCacheMutationVariables> = gql`
  mutation ClearSemanticCache($id: ID!) {
    clearSemanticCache(id: $id)
  }
`;

export const CLEAR_ALL_SEMANTIC_CACHES: TypedDocumentNode<ClearAllSemanticCachesMutation, ClearAllSemanticCachesMutationVariables> = gql`
  mutation ClearAllSemanticCaches {
    clearAllSemanticCaches
  }
`;

export const CACHE_CONFIG_QUERY: TypedDocumentNode<CacheConfigQuery, CacheConfigQueryVariables> = gql`
  query CacheConfig {
    cacheConfig {
      id isEnabled similarityThreshold defaultTtlMinutes embeddingModel maxCacheSize
    }
  }
`;

export const UPDATE_CACHE_CONFIG: TypedDocumentNode<UpdateCacheConfigMutation, UpdateCacheConfigMutationVariables> = gql`
  mutation UpdateCacheConfig($input: CacheConfigInput!) {
    updateCacheConfig(input: $input) {
      id isEnabled similarityThreshold defaultTtlMinutes embeddingModel maxCacheSize
    }
  }
`;

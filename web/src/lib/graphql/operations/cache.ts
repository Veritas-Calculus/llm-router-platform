import { gql } from '@apollo/client';

export const GET_SEMANTIC_CACHES = gql`
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

export const GET_CACHE_STATS = gql`
  query GetCacheStats {
    cacheStats {
      totalCaches
      totalHits
    }
  }
`;

export const CLEAR_SEMANTIC_CACHE = gql`
  mutation ClearSemanticCache($id: ID!) {
    clearSemanticCache(id: $id)
  }
`;

export const CLEAR_ALL_SEMANTIC_CACHES = gql`
  mutation ClearAllSemanticCaches {
    clearAllSemanticCaches
  }
`;

export const CACHE_CONFIG_QUERY = gql`
  query CacheConfig {
    cacheConfig {
      id isEnabled similarityThreshold defaultTtlMinutes embeddingModel maxCacheSize
    }
  }
`;

export const UPDATE_CACHE_CONFIG = gql`
  mutation UpdateCacheConfig($input: CacheConfigInput!) {
    updateCacheConfig(input: $input) {
      id isEnabled similarityThreshold defaultTtlMinutes embeddingModel maxCacheSize
    }
  }
`;

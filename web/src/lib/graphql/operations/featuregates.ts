import { gql } from '@apollo/client';

export const FEATURE_GATES_QUERY = gql`
  query FeatureGates {
    featureGates {
      name
      enabled
      category
      description
      envVar
      source
      locked
    }
  }
`;

export const UPDATE_FEATURE_GATE = gql`
  mutation UpdateFeatureGate($name: String!, $enabled: Boolean!) {
    updateFeatureGate(name: $name, enabled: $enabled) {
      name
      enabled
      category
      description
      envVar
      source
      locked
    }
  }
`;

import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  FeatureGatesQuery,
  FeatureGatesQueryVariables,
  UpdateFeatureGateMutation,
  UpdateFeatureGateMutationVariables,
} from '../generated/graphql';

export const FEATURE_GATES_QUERY: TypedDocumentNode<FeatureGatesQuery, FeatureGatesQueryVariables> = gql`
  query FeatureGates {
    featureGates {
      name
      enabled
      category
      description
      source
    }
  }
`;

export const UPDATE_FEATURE_GATE: TypedDocumentNode<UpdateFeatureGateMutation, UpdateFeatureGateMutationVariables> = gql`
  mutation UpdateFeatureGate($name: String!, $enabled: Boolean!) {
    updateFeatureGate(name: $name, enabled: $enabled) {
      name
      enabled
      category
      description
      source
    }
  }
`;

import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  GetIntegrationsQuery,
  GetIntegrationsQueryVariables,
  TestLangfuseConnectionMutation,
  TestLangfuseConnectionMutationVariables,
  UpdateIntegrationMutation,
  UpdateIntegrationMutationVariables,
} from '../generated/graphql';

export const GET_INTEGRATIONS: TypedDocumentNode<GetIntegrationsQuery, GetIntegrationsQueryVariables> = gql`
  query GetIntegrations {
    integrations {
      id
      name
      enabled
      config
      updatedAt
    }
  }
`;

export const UPDATE_INTEGRATION: TypedDocumentNode<UpdateIntegrationMutation, UpdateIntegrationMutationVariables> = gql`
  mutation UpdateIntegration($name: String!, $input: UpdateIntegrationInput!) {
    updateIntegration(name: $name, input: $input) {
      id
      name
      enabled
      config
      updatedAt
    }
  }
`;

export const TEST_LANGFUSE_CONNECTION: TypedDocumentNode<TestLangfuseConnectionMutation, TestLangfuseConnectionMutationVariables> = gql`
  mutation TestLangfuseConnection($publicKey: String!, $secretKey: String!, $host: String!) {
    testLangfuseConnection(publicKey: $publicKey, secretKey: $secretKey, host: $host)
  }
`;

import { gql } from '@apollo/client';

export const GET_INTEGRATIONS = gql`
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

export const UPDATE_INTEGRATION = gql`
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

export const TEST_LANGFUSE_CONNECTION = gql`
  mutation TestLangfuseConnection($publicKey: String!, $secretKey: String!, $host: String!) {
    testLangfuseConnection(publicKey: $publicKey, secretKey: $secretKey, host: $host)
  }
`;

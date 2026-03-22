import { gql } from '@apollo/client';

export const GET_DLP_CONFIG = gql`
  query GetDlpConfig($projectId: ID!) {
    getDlpConfig(projectId: $projectId) {
      id
      projectId
      isEnabled
      strategy
      maskEmails
      maskPhones
      maskCreditCards
      maskSsn
      maskApiKeys
      customRegex
      createdAt
      updatedAt
    }
  }
`;

export const UPDATE_DLP_CONFIG = gql`
  mutation UpdateDlpConfig($input: UpdateDlpConfigInput!) {
    updateDlpConfig(input: $input) {
      id
      projectId
      isEnabled
      strategy
      maskEmails
      maskPhones
      maskCreditCards
      maskSsn
      maskApiKeys
      customRegex
      updatedAt
    }
  }
`;

export const TEST_DLP_REDACTION = gql`
  query TestDlpRedaction($projectId: ID!, $input: String!) {
    testDlpRedaction(projectId: $projectId, input: $input) {
      originalText
      scrubbedText
      hasPii
      blocked
    }
  }
`;

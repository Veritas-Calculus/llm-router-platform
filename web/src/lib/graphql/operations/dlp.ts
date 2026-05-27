import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  GetDlpConfigQuery,
  GetDlpConfigQueryVariables,
  TestDlpRedactionQuery,
  TestDlpRedactionQueryVariables,
  UpdateDlpConfigMutation,
  UpdateDlpConfigMutationVariables,
} from '../generated/graphql';

export const GET_DLP_CONFIG: TypedDocumentNode<GetDlpConfigQuery, GetDlpConfigQueryVariables> = gql`
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

export const UPDATE_DLP_CONFIG: TypedDocumentNode<UpdateDlpConfigMutation, UpdateDlpConfigMutationVariables> = gql`
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

export const TEST_DLP_REDACTION: TypedDocumentNode<TestDlpRedactionQuery, TestDlpRedactionQueryVariables> = gql`
  query TestDlpRedaction($projectId: ID!, $input: String!) {
    testDlpRedaction(projectId: $projectId, input: $input) {
      originalText
      scrubbedText
      hasPii
      blocked
    }
  }
`;

import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  CreatePromptTemplateMutation,
  CreatePromptTemplateMutationVariables,
  CreatePromptVersionMutation,
  CreatePromptVersionMutationVariables,
  DeletePromptTemplateMutation,
  DeletePromptTemplateMutationVariables,
  PromptTemplateQuery,
  PromptTemplateQueryVariables,
  PromptTemplatesQuery,
  PromptTemplatesQueryVariables,
  PromptVersionsQuery,
  PromptVersionsQueryVariables,
  SetActivePromptVersionMutation,
  SetActivePromptVersionMutationVariables,
  UpdatePromptTemplateMutation,
  UpdatePromptTemplateMutationVariables,
} from '../generated/graphql';

// ── Prompt Template Operations ──────────────────────────────────────

export const PROMPT_TEMPLATES_QUERY: TypedDocumentNode<PromptTemplatesQuery, PromptTemplatesQueryVariables> = gql`
  query PromptTemplates {
    promptTemplates {
      data {
        id name description projectId isActive activeVersionId
        activeVersion { id version content model createdAt }
        versionCount createdAt updatedAt
      }
      total
    }
  }
`;

export const PROMPT_TEMPLATE_QUERY: TypedDocumentNode<PromptTemplateQuery, PromptTemplateQueryVariables> = gql`
  query PromptTemplate($id: ID!) {
    promptTemplate(id: $id) {
      id name description projectId isActive activeVersionId
      activeVersion { id version content model createdAt }
      versionCount createdAt updatedAt
    }
  }
`;

export const PROMPT_VERSIONS_QUERY: TypedDocumentNode<PromptVersionsQuery, PromptVersionsQueryVariables> = gql`
  query PromptVersions($templateId: ID!) {
    promptVersions(templateId: $templateId) {
      id templateId version content model parameters changeLog createdAt
    }
  }
`;

export const CREATE_PROMPT_TEMPLATE: TypedDocumentNode<CreatePromptTemplateMutation, CreatePromptTemplateMutationVariables> = gql`
  mutation CreatePromptTemplate($input: PromptTemplateInput!) {
    createPromptTemplate(input: $input) {
      id name description isActive versionCount createdAt updatedAt
    }
  }
`;

export const UPDATE_PROMPT_TEMPLATE: TypedDocumentNode<UpdatePromptTemplateMutation, UpdatePromptTemplateMutationVariables> = gql`
  mutation UpdatePromptTemplate($id: ID!, $input: PromptTemplateInput!) {
    updatePromptTemplate(id: $id, input: $input) {
      id name description isActive versionCount createdAt updatedAt
    }
  }
`;

export const DELETE_PROMPT_TEMPLATE: TypedDocumentNode<DeletePromptTemplateMutation, DeletePromptTemplateMutationVariables> = gql`
  mutation DeletePromptTemplate($id: ID!) {
    deletePromptTemplate(id: $id)
  }
`;

export const CREATE_PROMPT_VERSION: TypedDocumentNode<CreatePromptVersionMutation, CreatePromptVersionMutationVariables> = gql`
  mutation CreatePromptVersion($input: PromptVersionInput!) {
    createPromptVersion(input: $input) {
      id templateId version content model parameters changeLog createdAt
    }
  }
`;

export const SET_ACTIVE_PROMPT_VERSION: TypedDocumentNode<SetActivePromptVersionMutation, SetActivePromptVersionMutationVariables> = gql`
  mutation SetActivePromptVersion($templateId: ID!, $versionId: ID!) {
    setActivePromptVersion(templateId: $templateId, versionId: $versionId) {
      id activeVersionId
      activeVersion { id version content model createdAt }
    }
  }
`;

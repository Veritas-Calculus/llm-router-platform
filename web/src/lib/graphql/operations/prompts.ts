import { gql } from '@apollo/client';

// ── Prompt Template Operations ──────────────────────────────────────

export const PROMPT_TEMPLATES_QUERY = gql`
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

export const PROMPT_TEMPLATE_QUERY = gql`
  query PromptTemplate($id: ID!) {
    promptTemplate(id: $id) {
      id name description projectId isActive activeVersionId
      activeVersion { id version content model createdAt }
      versionCount createdAt updatedAt
    }
  }
`;

export const PROMPT_VERSIONS_QUERY = gql`
  query PromptVersions($templateId: ID!) {
    promptVersions(templateId: $templateId) {
      id templateId version content model parameters changeLog createdAt
    }
  }
`;

export const CREATE_PROMPT_TEMPLATE = gql`
  mutation CreatePromptTemplate($input: PromptTemplateInput!) {
    createPromptTemplate(input: $input) {
      id name description isActive versionCount createdAt updatedAt
    }
  }
`;

export const UPDATE_PROMPT_TEMPLATE = gql`
  mutation UpdatePromptTemplate($id: ID!, $input: PromptTemplateInput!) {
    updatePromptTemplate(id: $id, input: $input) {
      id name description isActive versionCount createdAt updatedAt
    }
  }
`;

export const DELETE_PROMPT_TEMPLATE = gql`
  mutation DeletePromptTemplate($id: ID!) {
    deletePromptTemplate(id: $id)
  }
`;

export const CREATE_PROMPT_VERSION = gql`
  mutation CreatePromptVersion($input: PromptVersionInput!) {
    createPromptVersion(input: $input) {
      id templateId version content model parameters changeLog createdAt
    }
  }
`;

export const SET_ACTIVE_PROMPT_VERSION = gql`
  mutation SetActivePromptVersion($templateId: ID!, $versionId: ID!) {
    setActivePromptVersion(templateId: $templateId, versionId: $versionId) {
      id activeVersionId
      activeVersion { id version content model createdAt }
    }
  }
`;

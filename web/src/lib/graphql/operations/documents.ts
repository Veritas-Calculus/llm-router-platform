import { gql } from '@apollo/client';

// ── Admin: Documents ──────────────────────────────────────────

export const DOCUMENTS_QUERY = gql`
  query Documents {
    documents {
      id
      title
      slug
      content
      category
      sortOrder
      isPublished
      createdAt
      updatedAt
    }
  }
`;

export const CREATE_DOCUMENT = gql`
  mutation CreateDocument($input: DocumentInput!) {
    createDocument(input: $input) {
      id
      title
      slug
      content
      category
      sortOrder
      isPublished
      createdAt
    }
  }
`;

export const UPDATE_DOCUMENT = gql`
  mutation UpdateDocument($id: ID!, $input: DocumentInput!) {
    updateDocument(id: $id, input: $input) {
      id
      title
      slug
      content
      category
      sortOrder
      isPublished
      updatedAt
    }
  }
`;

export const DELETE_DOCUMENT = gql`
  mutation DeleteDocument($id: ID!) {
    deleteDocument(id: $id)
  }
`;

import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  CreateDocumentMutation,
  CreateDocumentMutationVariables,
  DeleteDocumentMutation,
  DeleteDocumentMutationVariables,
  DocumentsQuery,
  DocumentsQueryVariables,
  PublishedDocumentsQuery,
  PublishedDocumentsQueryVariables,
  UpdateDocumentMutation,
  UpdateDocumentMutationVariables,
} from '../generated/graphql';

// ── Admin: Documents ──────────────────────────────────────────

export const DOCUMENTS_QUERY: TypedDocumentNode<DocumentsQuery, DocumentsQueryVariables> = gql`
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

export const PUBLISHED_DOCUMENTS_QUERY: TypedDocumentNode<PublishedDocumentsQuery, PublishedDocumentsQueryVariables> = gql`
  query PublishedDocuments {
    publishedDocuments {
      id
      title
      slug
      content
      category
      sortOrder
      updatedAt
    }
  }
`;

export const CREATE_DOCUMENT: TypedDocumentNode<CreateDocumentMutation, CreateDocumentMutationVariables> = gql`
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

export const UPDATE_DOCUMENT: TypedDocumentNode<UpdateDocumentMutation, UpdateDocumentMutationVariables> = gql`
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

export const DELETE_DOCUMENT: TypedDocumentNode<DeleteDocumentMutation, DeleteDocumentMutationVariables> = gql`
  mutation DeleteDocument($id: ID!) {
    deleteDocument(id: $id)
  }
`;

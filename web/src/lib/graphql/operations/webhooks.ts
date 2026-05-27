import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  CreateWebhookEndpointMutation,
  CreateWebhookEndpointMutationVariables,
  DeleteWebhookEndpointMutation,
  DeleteWebhookEndpointMutationVariables,
  GetWebhookDeliveriesQuery,
  GetWebhookDeliveriesQueryVariables,
  GetWebhooksQuery,
  GetWebhooksQueryVariables,
  TestWebhookEndpointMutation,
  TestWebhookEndpointMutationVariables,
  UpdateWebhookEndpointMutation,
  UpdateWebhookEndpointMutationVariables,
} from '../generated/graphql';

export const WEBHOOK_ENDPOINT_FIELDS = gql`
  fragment WebhookEndpointFields on WebhookEndpoint {
    id
    projectId
    url
    events
    isActive
    description
    createdAt
    updatedAt
  }
`;

export const GET_WEBHOOKS: TypedDocumentNode<GetWebhooksQuery, GetWebhooksQueryVariables> = gql`
  ${WEBHOOK_ENDPOINT_FIELDS}
  query GetWebhooks($projectId: ID!) {
    webhooks(projectId: $projectId) {
      ...WebhookEndpointFields
    }
  }
`;

export const CREATE_WEBHOOK_ENDPOINT: TypedDocumentNode<CreateWebhookEndpointMutation, CreateWebhookEndpointMutationVariables> = gql`
  ${WEBHOOK_ENDPOINT_FIELDS}
  mutation CreateWebhookEndpoint($input: CreateWebhookEndpointInput!) {
    createWebhookEndpoint(input: $input) {
      ...WebhookEndpointFields
      secret
    }
  }
`;

export const UPDATE_WEBHOOK_ENDPOINT: TypedDocumentNode<UpdateWebhookEndpointMutation, UpdateWebhookEndpointMutationVariables> = gql`
  ${WEBHOOK_ENDPOINT_FIELDS}
  mutation UpdateWebhookEndpoint($id: ID!, $input: UpdateWebhookEndpointInput!) {
    updateWebhookEndpoint(id: $id, input: $input) {
      ...WebhookEndpointFields
    }
  }
`;

export const DELETE_WEBHOOK_ENDPOINT: TypedDocumentNode<DeleteWebhookEndpointMutation, DeleteWebhookEndpointMutationVariables> = gql`
  mutation DeleteWebhookEndpoint($id: ID!) {
    deleteWebhookEndpoint(id: $id)
  }
`;

export const TEST_WEBHOOK_ENDPOINT: TypedDocumentNode<TestWebhookEndpointMutation, TestWebhookEndpointMutationVariables> = gql`
  mutation TestWebhookEndpoint($id: ID!) {
    testWebhookEndpoint(id: $id)
  }
`;

export const GET_WEBHOOK_DELIVERIES: TypedDocumentNode<GetWebhookDeliveriesQuery, GetWebhookDeliveriesQueryVariables> = gql`
  query GetWebhookDeliveries($endpointId: ID!, $limit: Int) {
    webhookDeliveries(endpointId: $endpointId, limit: $limit) {
      id
      endpointId
      eventType
      payload
      status
      statusCode
      responseBody
      errorMessage
      retryCount
      createdAt
      updatedAt
    }
  }
`;

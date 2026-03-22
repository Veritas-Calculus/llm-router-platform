import { gql } from '@apollo/client';

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

export const GET_WEBHOOKS = gql`
  ${WEBHOOK_ENDPOINT_FIELDS}
  query GetWebhooks($projectId: ID!) {
    webhooks(projectId: $projectId) {
      ...WebhookEndpointFields
    }
  }
`;

export const CREATE_WEBHOOK_ENDPOINT = gql`
  ${WEBHOOK_ENDPOINT_FIELDS}
  mutation CreateWebhookEndpoint($input: CreateWebhookEndpointInput!) {
    createWebhookEndpoint(input: $input) {
      ...WebhookEndpointFields
      secret
    }
  }
`;

export const UPDATE_WEBHOOK_ENDPOINT = gql`
  ${WEBHOOK_ENDPOINT_FIELDS}
  mutation UpdateWebhookEndpoint($id: ID!, $input: UpdateWebhookEndpointInput!) {
    updateWebhookEndpoint(id: $id, input: $input) {
      ...WebhookEndpointFields
    }
  }
`;

export const DELETE_WEBHOOK_ENDPOINT = gql`
  mutation DeleteWebhookEndpoint($id: ID!) {
    deleteWebhookEndpoint(id: $id)
  }
`;

export const TEST_WEBHOOK_ENDPOINT = gql`
  mutation TestWebhookEndpoint($id: ID!) {
    testWebhookEndpoint(id: $id)
  }
`;

export const GET_WEBHOOK_DELIVERIES = gql`
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

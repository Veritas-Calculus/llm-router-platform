import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  CreateNotificationChannelMutation,
  CreateNotificationChannelMutationVariables,
  DeleteNotificationChannelMutation,
  DeleteNotificationChannelMutationVariables,
  NotificationChannelsQuery,
  NotificationChannelsQueryVariables,
  TestNotificationChannelMutation,
  TestNotificationChannelMutationVariables,
  UpdateNotificationChannelMutation,
  UpdateNotificationChannelMutationVariables,
} from '../generated/graphql';

export const NOTIFICATION_CHANNELS_QUERY: TypedDocumentNode<NotificationChannelsQuery, NotificationChannelsQueryVariables> = gql`
  query NotificationChannels {
    notificationChannels {
      id name type isEnabled config createdAt updatedAt
    }
  }
`;

export const CREATE_NOTIFICATION_CHANNEL: TypedDocumentNode<CreateNotificationChannelMutation, CreateNotificationChannelMutationVariables> = gql`
  mutation CreateNotificationChannel($input: NotificationChannelInput!) {
    createNotificationChannel(input: $input) {
      id name type isEnabled config createdAt
    }
  }
`;

export const UPDATE_NOTIFICATION_CHANNEL: TypedDocumentNode<UpdateNotificationChannelMutation, UpdateNotificationChannelMutationVariables> = gql`
  mutation UpdateNotificationChannel($id: ID!, $input: UpdateNotificationChannelInput!) {
    updateNotificationChannel(id: $id, input: $input) {
      id name type isEnabled config
    }
  }
`;

export const DELETE_NOTIFICATION_CHANNEL: TypedDocumentNode<DeleteNotificationChannelMutation, DeleteNotificationChannelMutationVariables> = gql`
  mutation DeleteNotificationChannel($id: ID!) {
    deleteNotificationChannel(id: $id)
  }
`;

export const TEST_NOTIFICATION_CHANNEL: TypedDocumentNode<TestNotificationChannelMutation, TestNotificationChannelMutationVariables> = gql`
  mutation TestNotificationChannel($id: ID!) {
    testNotificationChannel(id: $id)
  }
`;

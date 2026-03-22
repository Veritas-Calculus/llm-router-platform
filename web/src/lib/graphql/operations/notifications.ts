import { gql } from '@apollo/client';

export const NOTIFICATION_CHANNELS_QUERY = gql`
  query NotificationChannels {
    notificationChannels {
      id name type isEnabled config createdAt updatedAt
    }
  }
`;

export const CREATE_NOTIFICATION_CHANNEL = gql`
  mutation CreateNotificationChannel($input: NotificationChannelInput!) {
    createNotificationChannel(input: $input) {
      id name type isEnabled config createdAt
    }
  }
`;

export const UPDATE_NOTIFICATION_CHANNEL = gql`
  mutation UpdateNotificationChannel($id: ID!, $input: UpdateNotificationChannelInput!) {
    updateNotificationChannel(id: $id, input: $input) {
      id name type isEnabled config
    }
  }
`;

export const DELETE_NOTIFICATION_CHANNEL = gql`
  mutation DeleteNotificationChannel($id: ID!) {
    deleteNotificationChannel(id: $id)
  }
`;

export const TEST_NOTIFICATION_CHANNEL = gql`
  mutation TestNotificationChannel($id: ID!) {
    testNotificationChannel(id: $id)
  }
`;

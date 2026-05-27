import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  ActiveAnnouncementsQuery,
  ActiveAnnouncementsQueryVariables,
  AnnouncementsQuery,
  AnnouncementsQueryVariables,
  CreateAnnouncementMutation,
  CreateAnnouncementMutationVariables,
  DeleteAnnouncementMutation,
  DeleteAnnouncementMutationVariables,
  UpdateAnnouncementMutation,
  UpdateAnnouncementMutationVariables,
} from '../generated/graphql';

// ── Admin: Announcements ──────────────────────────────────────────

export const ANNOUNCEMENTS_QUERY: TypedDocumentNode<AnnouncementsQuery, AnnouncementsQueryVariables> = gql`
  query Announcements {
    announcements {
      id
      title
      content
      type
      priority
      isActive
      startsAt
      endsAt
      createdAt
      updatedAt
    }
  }
`;

export const CREATE_ANNOUNCEMENT: TypedDocumentNode<CreateAnnouncementMutation, CreateAnnouncementMutationVariables> = gql`
  mutation CreateAnnouncement($input: AnnouncementInput!) {
    createAnnouncement(input: $input) {
      id
      title
      content
      type
      priority
      isActive
      startsAt
      endsAt
      createdAt
    }
  }
`;

export const UPDATE_ANNOUNCEMENT: TypedDocumentNode<UpdateAnnouncementMutation, UpdateAnnouncementMutationVariables> = gql`
  mutation UpdateAnnouncement($id: ID!, $input: AnnouncementInput!) {
    updateAnnouncement(id: $id, input: $input) {
      id
      title
      content
      type
      priority
      isActive
      startsAt
      endsAt
      updatedAt
    }
  }
`;

export const DELETE_ANNOUNCEMENT: TypedDocumentNode<DeleteAnnouncementMutation, DeleteAnnouncementMutationVariables> = gql`
  mutation DeleteAnnouncement($id: ID!) {
    deleteAnnouncement(id: $id)
  }
`;

// ── User-facing: Active Announcements ─────────────────────────────

export const ACTIVE_ANNOUNCEMENTS_QUERY: TypedDocumentNode<ActiveAnnouncementsQuery, ActiveAnnouncementsQueryVariables> = gql`
  query ActiveAnnouncements {
    activeAnnouncements {
      id
      title
      content
      type
      priority
      startsAt
      endsAt
      createdAt
    }
  }
`;

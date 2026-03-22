import { gql } from '@apollo/client';

// ── Admin: Announcements ──────────────────────────────────────────

export const ANNOUNCEMENTS_QUERY = gql`
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

export const CREATE_ANNOUNCEMENT = gql`
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

export const UPDATE_ANNOUNCEMENT = gql`
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

export const DELETE_ANNOUNCEMENT = gql`
  mutation DeleteAnnouncement($id: ID!) {
    deleteAnnouncement(id: $id)
  }
`;

// ── User-facing: Active Announcements ─────────────────────────────

export const ACTIVE_ANNOUNCEMENTS_QUERY = gql`
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

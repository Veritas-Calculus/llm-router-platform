import { gql } from '@apollo/client';

// ── Admin: Plans ──────────────────────────────────────────

export const PLANS_QUERY = gql`
  query Plans {
    plans {
      id
      name
      description
      priceMonth
      tokenLimit
      rateLimit
      supportLevel
      features
      isActive
    }
  }
`;

export const CREATE_PLAN = gql`
  mutation CreatePlan($input: PlanInput!) {
    createPlan(input: $input) {
      id
      name
      priceMonth
      tokenLimit
      rateLimit
      features
      isActive
    }
  }
`;

export const UPDATE_PLAN = gql`
  mutation UpdatePlan($id: ID!, $input: PlanInput!) {
    updatePlan(id: $id, input: $input) {
      id
      name
      priceMonth
      tokenLimit
      rateLimit
      features
      isActive
    }
  }
`;

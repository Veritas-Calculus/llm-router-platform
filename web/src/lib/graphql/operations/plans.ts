import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  CreatePlanMutation,
  CreatePlanMutationVariables,
  PlansQuery,
  PlansQueryVariables,
  UpdatePlanMutation,
  UpdatePlanMutationVariables,
} from '../generated/graphql';

// ── Admin: Plans ──────────────────────────────────────────

export const PLANS_QUERY: TypedDocumentNode<PlansQuery, PlansQueryVariables> = gql`
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

export const CREATE_PLAN: TypedDocumentNode<CreatePlanMutation, CreatePlanMutationVariables> = gql`
  mutation CreatePlan($input: PlanInput!) {
    createPlan(input: $input) {
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

export const UPDATE_PLAN: TypedDocumentNode<UpdatePlanMutation, UpdatePlanMutationVariables> = gql`
  mutation UpdatePlan($id: ID!, $input: PlanInput!) {
    updatePlan(id: $id, input: $input) {
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

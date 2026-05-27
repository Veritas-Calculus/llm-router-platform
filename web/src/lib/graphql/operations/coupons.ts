import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  CouponsQuery,
  CouponsQueryVariables,
  CreateCouponMutation,
  CreateCouponMutationVariables,
  DeleteCouponMutation,
  DeleteCouponMutationVariables,
  UpdateCouponMutation,
  UpdateCouponMutationVariables,
} from '../generated/graphql';

// ── Admin: Coupons ──────────────────────────────────────────

export const COUPONS_QUERY: TypedDocumentNode<CouponsQuery, CouponsQueryVariables> = gql`
  query Coupons {
    coupons {
      id
      code
      name
      type
      discountValue
      minAmount
      maxUses
      useCount
      maxUsesPerUser
      isActive
      expiresAt
      createdAt
    }
  }
`;

export const CREATE_COUPON: TypedDocumentNode<CreateCouponMutation, CreateCouponMutationVariables> = gql`
  mutation CreateCoupon($input: CouponInput!) {
    createCoupon(input: $input) {
      id
      code
      name
      type
      discountValue
      minAmount
      maxUses
      maxUsesPerUser
      isActive
      expiresAt
      createdAt
    }
  }
`;

export const UPDATE_COUPON: TypedDocumentNode<UpdateCouponMutation, UpdateCouponMutationVariables> = gql`
  mutation UpdateCoupon($id: ID!, $input: CouponInput!) {
    updateCoupon(id: $id, input: $input) {
      id
      code
      name
      type
      discountValue
      minAmount
      maxUses
      maxUsesPerUser
      isActive
      expiresAt
    }
  }
`;

export const DELETE_COUPON: TypedDocumentNode<DeleteCouponMutation, DeleteCouponMutationVariables> = gql`
  mutation DeleteCoupon($id: ID!) {
    deleteCoupon(id: $id)
  }
`;

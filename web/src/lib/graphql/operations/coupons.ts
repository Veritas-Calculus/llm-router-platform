import { gql } from '@apollo/client';

// ── Admin: Coupons ──────────────────────────────────────────

export const COUPONS_QUERY = gql`
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

export const CREATE_COUPON = gql`
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

export const UPDATE_COUPON = gql`
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

export const DELETE_COUPON = gql`
  mutation DeleteCoupon($id: ID!) {
    deleteCoupon(id: $id)
  }
`;

import { gql } from '@apollo/client';

// ── Redeem Code Operations ──────────────────────────────────────────

export const REDEEM_CODE_MUTATION = gql`
  mutation RedeemCode($code: String!) {
    redeemCode(code: $code) {
      success
      message
      creditAmount
      planName
    }
  }
`;

export const MY_REDEEM_HISTORY = gql`
  query MyRedeemHistory {
    myRedeemHistory {
      id
      code
      creditAmount
      planName
      redeemedAt
    }
  }
`;

// ── Admin: Redeem Codes ──
export const ADMIN_REDEEM_CODES_QUERY = gql`
  query AdminRedeemCodes($page: Int, $pageSize: Int) {
    redeemCodes(page: $page, pageSize: $pageSize) {
      nodes {
        id
        code
        type
        creditAmount
        planId
        usedBy
        usedAt
        expiresAt
        isActive
        createdAt
      }
      total
    }
  }
`;

export const GENERATE_REDEEM_CODES = gql`
  mutation GenerateRedeemCodes($input: GenerateRedeemCodesInput!) {
    generateRedeemCodes(input: $input) {
      codes
      count
    }
  }
`;

export const REVOKE_REDEEM_CODE = gql`
  mutation RevokeRedeemCode($id: ID!) {
    revokeRedeemCode(id: $id)
  }
`;

import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  AdminRedeemCodesQuery,
  AdminRedeemCodesQueryVariables,
  GenerateRedeemCodesMutation,
  GenerateRedeemCodesMutationVariables,
  MyRedeemHistoryQuery,
  MyRedeemHistoryQueryVariables,
  RedeemCodeMutation,
  RedeemCodeMutationVariables,
  RevokeRedeemCodeMutation,
  RevokeRedeemCodeMutationVariables,
} from '../generated/graphql';

// ── Redeem Code Operations ──────────────────────────────────────────

export const REDEEM_CODE_MUTATION: TypedDocumentNode<RedeemCodeMutation, RedeemCodeMutationVariables> = gql`
  mutation RedeemCode($code: String!) {
    redeemCode(code: $code) {
      success
      message
      creditAmount
      planName
    }
  }
`;

export const MY_REDEEM_HISTORY: TypedDocumentNode<MyRedeemHistoryQuery, MyRedeemHistoryQueryVariables> = gql`
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
export const ADMIN_REDEEM_CODES_QUERY: TypedDocumentNode<AdminRedeemCodesQuery, AdminRedeemCodesQueryVariables> = gql`
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

export const GENERATE_REDEEM_CODES: TypedDocumentNode<GenerateRedeemCodesMutation, GenerateRedeemCodesMutationVariables> = gql`
  mutation GenerateRedeemCodes($input: GenerateRedeemCodesInput!) {
    generateRedeemCodes(input: $input) {
      codes
      count
    }
  }
`;

export const REVOKE_REDEEM_CODE: TypedDocumentNode<RevokeRedeemCodeMutation, RevokeRedeemCodeMutationVariables> = gql`
  mutation RevokeRedeemCode($id: ID!) {
    revokeRedeemCode(id: $id)
  }
`;

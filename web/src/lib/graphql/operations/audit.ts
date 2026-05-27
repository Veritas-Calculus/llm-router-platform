import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  GetAuditLogsQuery,
  GetAuditLogsQueryVariables,
} from '../generated/graphql';

export const GET_AUDIT_LOGS: TypedDocumentNode<GetAuditLogsQuery, GetAuditLogsQueryVariables> = gql`
  query GetAuditLogs($page: Int, $pageSize: Int, $action: String) {
    auditLogs(page: $page, pageSize: $pageSize, action: $action) {
      data {
        id
        createdAt
        action
        actorId
        targetId
        ip
        userAgent
        detail
      }
      total
      page
      pageSize
    }
  }
`;

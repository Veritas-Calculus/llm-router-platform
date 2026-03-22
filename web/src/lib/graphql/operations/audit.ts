import { gql } from '@apollo/client';

export const GET_AUDIT_LOGS = gql`
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

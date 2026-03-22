import { gql } from '@apollo/client';

export const GET_ERROR_LOGS = gql`
  query GetErrorLogs($page: Int, $pageSize: Int) {
    errorLogs(page: $page, pageSize: $pageSize) {
      data {
        id
        trajectoryId
        traceId
        provider
        model
        statusCode
        headers
        responseBody
        createdAt
      }
      total
      page
      pageSize
    }
  }
`;

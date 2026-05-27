import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  GetErrorLogsQuery,
  GetErrorLogsQueryVariables,
  GetRequestLogsQuery,
  GetRequestLogsQueryVariables,
} from '../generated/graphql';

export const GET_ERROR_LOGS: TypedDocumentNode<GetErrorLogsQuery, GetErrorLogsQueryVariables> = gql`
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

export const GET_REQUEST_LOGS: TypedDocumentNode<GetRequestLogsQuery, GetRequestLogsQueryVariables> = gql`
  query GetRequestLogs($requestId: String, $level: String, $startTime: String, $endTime: String, $limit: Int) {
    requestLogs(requestId: $requestId, level: $level, startTime: $startTime, endTime: $endTime, limit: $limit) {
      timestamp
      level
      message
      requestId
      caller
      error
      method
      path
      statusCode
      latency
      clientIp
      userAgent
      rawJson
    }
  }
`;

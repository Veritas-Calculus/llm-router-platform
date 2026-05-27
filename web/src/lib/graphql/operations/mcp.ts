import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  CreateMcpServerMutation,
  CreateMcpServerMutationVariables,
  DeleteMcpServerMutation,
  DeleteMcpServerMutationVariables,
  McpServerDetailQuery,
  McpServerDetailQueryVariables,
  McpServersQuery,
  McpServersQueryVariables,
  McpToolsQuery,
  McpToolsQueryVariables,
  RefreshMcpToolsMutation,
  RefreshMcpToolsMutationVariables,
  UpdateMcpServerMutation,
  UpdateMcpServerMutationVariables,
} from '../generated/graphql';

// ── MCP Operations ──────────────────────────────────────────────────

export const MCP_SERVERS_QUERY: TypedDocumentNode<McpServersQuery, McpServersQueryVariables> = gql`
  query McpServers {
    mcpServers {
      id name url type command args isActive status lastError lastCheckedAt createdAt
      tools { id serverId name description inputSchema isActive }
    }
  }
`;

export const MCP_SERVER_DETAIL_QUERY: TypedDocumentNode<McpServerDetailQuery, McpServerDetailQueryVariables> = gql`
  query McpServerDetail($id: ID!) {
    mcpServer(id: $id) {
      id name url type command args isActive status lastError lastCheckedAt createdAt
      tools { id serverId name description inputSchema isActive }
    }
  }
`;

export const MCP_TOOLS_QUERY: TypedDocumentNode<McpToolsQuery, McpToolsQueryVariables> = gql`
  query McpTools {
    mcpTools { id serverId name description inputSchema isActive }
  }
`;

export const CREATE_MCP_SERVER: TypedDocumentNode<CreateMcpServerMutation, CreateMcpServerMutationVariables> = gql`
  mutation CreateMcpServer($input: McpServerInput!) {
    createMcpServer(input: $input) {
      id name url type command args isActive status lastError lastCheckedAt createdAt
      tools { id serverId name description inputSchema isActive }
    }
  }
`;

export const UPDATE_MCP_SERVER: TypedDocumentNode<UpdateMcpServerMutation, UpdateMcpServerMutationVariables> = gql`
  mutation UpdateMcpServer($id: ID!, $input: McpServerInput!) {
    updateMcpServer(id: $id, input: $input) {
      id name url type command args isActive status lastError lastCheckedAt createdAt
      tools { id serverId name description inputSchema isActive }
    }
  }
`;

export const DELETE_MCP_SERVER: TypedDocumentNode<DeleteMcpServerMutation, DeleteMcpServerMutationVariables> = gql`
  mutation DeleteMcpServer($id: ID!) {
    deleteMcpServer(id: $id)
  }
`;

export const REFRESH_MCP_TOOLS: TypedDocumentNode<RefreshMcpToolsMutation, RefreshMcpToolsMutationVariables> = gql`
  mutation RefreshMcpTools($id: ID!) {
    refreshMcpTools(id: $id) {
      id name url type command args isActive status lastError lastCheckedAt createdAt
      tools { id serverId name description inputSchema isActive }
    }
  }
`;

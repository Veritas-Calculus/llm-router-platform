import { gql } from '@apollo/client';

// ── MCP Operations ──────────────────────────────────────────────────

export const MCP_SERVERS_QUERY = gql`
  query McpServers {
    mcpServers {
      id name url type command args isActive status lastError lastCheckedAt createdAt
      tools { id serverId name description inputSchema isActive }
    }
  }
`;

export const MCP_SERVER_DETAIL_QUERY = gql`
  query McpServerDetail($id: ID!) {
    mcpServer(id: $id) {
      id name url type command args isActive status lastError lastCheckedAt createdAt
      tools { id serverId name description inputSchema isActive }
    }
  }
`;

export const MCP_TOOLS_QUERY = gql`
  query McpTools {
    mcpTools { id serverId name description inputSchema isActive }
  }
`;

export const CREATE_MCP_SERVER = gql`
  mutation CreateMcpServer($input: McpServerInput!) {
    createMcpServer(input: $input) {
      id name url type command args isActive status lastError lastCheckedAt createdAt
      tools { id serverId name description inputSchema isActive }
    }
  }
`;

export const UPDATE_MCP_SERVER = gql`
  mutation UpdateMcpServer($id: ID!, $input: McpServerInput!) {
    updateMcpServer(id: $id, input: $input) {
      id name url type command args isActive status lastError lastCheckedAt createdAt
      tools { id serverId name description inputSchema isActive }
    }
  }
`;

export const DELETE_MCP_SERVER = gql`
  mutation DeleteMcpServer($id: ID!) {
    deleteMcpServer(id: $id)
  }
`;

export const REFRESH_MCP_TOOLS = gql`
  mutation RefreshMcpTools($id: ID!) {
    refreshMcpTools(id: $id) {
      id name url type command args isActive status lastError lastCheckedAt createdAt
      tools { id serverId name description inputSchema isActive }
    }
  }
`;

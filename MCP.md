# MCP (Model Context Protocol) Integration

The LLM Router Platform acts as an **MCP Host**, allowing you to extend the capabilities of any LLM model (OpenAI, Claude, Gemini, etc.) with external tools and data sources.

## Core Features

- **Multi-Transport Support**: Connect to MCP servers via `stdio` (local processes) or `sse` (remote HTTP services).
- **Auto-Injection**: Discovered tools are automatically injected into LLM requests as OpenAI-compatible functions.
- **Agentic Routing**: The platform intercepts tool calls from the model, executes them via the appropriate MCP server, and feeds the results back to the model in a feedback loop.
- **Observability**: Track tool call counts and error rates in the dashboard.
- **Conversation Memory**: Tool execution results are persisted in conversation history for context continuity.

## Configuration

### 1. Adding a Stdio Server (Local)
Useful for tools running on the same host or accessible via CLI.
- **Transport**: `stdio`
- **Command**: The executable (e.g., `npx`, `python`, `node`).
- **Arguments**: CLI arguments passed to the command.
- *Example (Google Search)*:
  - Command: `npx`
  - Args: `-y, @modelcontextprotocol/server-google-search`

### 2. Adding an SSE Server (Remote)
Useful for microservices or cloud-hosted tools.
- **Transport**: `sse`
- **URL**: The endpoint of the SSE service (e.g., `https://mcp-server.example.com/sse`).

## How it Works

1. **Discovery**: When an MCP server is added or refreshed, the platform calls `tools/list` to discover available tools and their JSON schemas.
2. **Request Enrichment**: When a chat request is made, if no tools are explicitly provided by the client, the platform injects all active MCP tools into the request.
3. **Execution Loop**:
   - Model returns a `tool_call` with a name like `server_name__tool_name`.
   - Router identifies the prefix and routes the call to the specific MCP server.
   - The result is formatted as a `tool` message and added to the message list.
   - The LLM is called again with the tool result until it provides a final response.

## Monitoring

- **Dashboard**: View "MCP Tool Calls" and "Errors" in the system health row.
- **MCP Management**: View status (Connected/Error), last error messages, and the list of discovered tools for each server.

## Future Roadmap

- **Granular Permissions**: Restrict specific MCP tools to specific API Keys or Users.
- **Model-Facing Resources**: Allow models to explicitly request MCP resources via a built-in `fetch_resource` tool.
- **Prompt Templates**: Support for MCP-native prompt templates.

import { useState } from 'react';
import { motion } from 'framer-motion';
import { ChevronRightIcon } from '@heroicons/react/24/outline';

interface DocSection {
  id: string;
  title: string;
  content: React.ReactNode;
}

// Styled components for documentation
const DocH3 = ({ children }: { children: React.ReactNode }) => (
  <h3 className="text-lg font-semibold text-apple-gray-900 mt-6 mb-3 first:mt-0">{children}</h3>
);

const DocH4 = ({ children }: { children: React.ReactNode }) => (
  <h4 className="text-base font-semibold text-apple-gray-800 mt-5 mb-2">{children}</h4>
);

const DocP = ({ children, className = '' }: { children: React.ReactNode; className?: string }) => (
  <p className={`text-apple-gray-600 mb-4 leading-relaxed ${className}`}>{children}</p>
);

const DocUl = ({ children }: { children: React.ReactNode }) => (
  <ul className="list-disc list-inside space-y-2 mb-4 text-apple-gray-600">{children}</ul>
);

const DocOl = ({ children }: { children: React.ReactNode }) => (
  <ol className="list-decimal list-inside space-y-2 mb-4 text-apple-gray-600">{children}</ol>
);

const DocLi = ({ children }: { children: React.ReactNode }) => (
  <li className="leading-relaxed">{children}</li>
);

const DocCode = ({ children }: { children: React.ReactNode }) => (
  <code className="bg-apple-gray-100 text-apple-gray-800 px-1.5 py-0.5 rounded text-sm font-mono">{children}</code>
);

const DocPre = ({ children }: { children: string }) => (
  <pre className="bg-apple-gray-900 text-apple-gray-100 p-4 rounded-apple overflow-x-auto mb-4 text-sm font-mono leading-relaxed">
    <code>{children}</code>
  </pre>
);

const DocTable = ({ children }: { children: React.ReactNode }) => (
  <div className="overflow-x-auto mb-4">
    <table className="min-w-full divide-y divide-apple-gray-200 text-sm">{children}</table>
  </div>
);

const DocTh = ({ children }: { children: React.ReactNode }) => (
  <th className="px-4 py-3 text-left font-semibold text-apple-gray-900 bg-apple-gray-50">{children}</th>
);

const DocTd = ({ children }: { children: React.ReactNode }) => (
  <td className="px-4 py-3 text-apple-gray-600 border-t border-apple-gray-100">{children}</td>
);

const DocStrong = ({ children }: { children: React.ReactNode }) => (
  <strong className="font-semibold text-apple-gray-800">{children}</strong>
);

const sections: DocSection[] = [
  {
    id: 'getting-started',
    title: 'Getting Started',
    content: (
      <div>
        <DocH3>What is LLM Router?</DocH3>
        <DocP>
          LLM Router is a unified API gateway for multiple Large Language Model providers. 
          It provides a single endpoint that intelligently routes requests to different LLM 
          providers (OpenAI, Anthropic, Google, Ollama, LM Studio, etc.) based on configured 
          priorities, weights, and availability.
        </DocP>

        <DocH3>Key Features</DocH3>
        <DocUl>
          <DocLi><DocStrong>Multi-Provider Support</DocStrong> - Connect to OpenAI, Anthropic, Google AI, Ollama, LM Studio and more</DocLi>
          <DocLi><DocStrong>Intelligent Routing</DocStrong> - Automatic failover and load balancing across providers</DocLi>
          <DocLi><DocStrong>Proxy Support</DocStrong> - Route requests through HTTP/SOCKS5 proxies for providers that require it</DocLi>
          <DocLi><DocStrong>API Key Management</DocStrong> - Manage multiple API keys per provider with rotation</DocLi>
          <DocLi><DocStrong>Health Monitoring</DocStrong> - Real-time health checks and alerting</DocLi>
          <DocLi><DocStrong>Usage Analytics</DocStrong> - Track token usage, costs, and request metrics</DocLi>
        </DocUl>

        <DocH3>Architecture Overview</DocH3>
        <DocP>The platform consists of three main components:</DocP>
        <DocUl>
          <DocLi><DocStrong>Router Service</DocStrong> - Go backend that handles API routing, health checks, and provider management</DocLi>
          <DocLi><DocStrong>Web Dashboard</DocStrong> - React frontend for configuration and monitoring</DocLi>
          <DocLi><DocStrong>Database</DocStrong> - PostgreSQL for persistent storage, Redis for caching</DocLi>
        </DocUl>
      </div>
    ),
  },
  {
    id: 'local-setup',
    title: 'Local Development Setup',
    content: (
      <div>
        <DocH3>Prerequisites</DocH3>
        <DocUl>
          <DocLi>Docker and Docker Compose</DocLi>
          <DocLi>Git</DocLi>
          <DocLi>(Optional) Go 1.25+ for backend development</DocLi>
          <DocLi>(Optional) Node.js 25+ for frontend development</DocLi>
        </DocUl>

        <DocH3>Quick Start with Docker</DocH3>
        <DocP>The easiest way to run the platform locally:</DocP>
        <DocPre>{`# Clone the repository
git clone https://github.com/Veritas-Calculus/llm-router-platform.git
cd llm-router-platform

# Start all services
docker compose up -d

# View logs
docker compose logs -f`}</DocPre>

        <DocP>Services will be available at:</DocP>
        <DocUl>
          <DocLi><DocStrong>Web Dashboard</DocStrong>: http://localhost:5173</DocLi>
          <DocLi><DocStrong>API Server</DocStrong>: http://localhost:8080</DocLi>
          <DocLi><DocStrong>PostgreSQL</DocStrong>: localhost:5432</DocLi>
          <DocLi><DocStrong>Redis</DocStrong>: localhost:6379</DocLi>
        </DocUl>

        <DocH3>Default Credentials</DocH3>
        <DocP>The system comes with a default admin account:</DocP>
        <DocUl>
          <DocLi><DocStrong>Email</DocStrong>: admin@example.com</DocLi>
          <DocLi><DocStrong>Password</DocStrong>: admin123</DocLi>
        </DocUl>
        <DocP className="text-apple-red font-medium">
          Important: Change these credentials immediately in a production environment.
        </DocP>

        <DocH3>Development Mode</DocH3>
        <DocP>For active development with hot reload:</DocP>
        <DocPre>{`# Start backend dependencies only
docker compose up -d postgres redis

# Run backend (in server directory)
cd server
go run ./cmd/server

# Run frontend (in web directory)
cd web
npm install
npm run dev`}</DocPre>
      </div>
    ),
  },
  {
    id: 'provider-config',
    title: 'Provider Configuration',
    content: (
      <div>
        <DocH3>Supported Providers</DocH3>
        <DocTable>
          <thead>
            <tr>
              <DocTh>Provider</DocTh>
              <DocTh>API Key Required</DocTh>
              <DocTh>Default Endpoint</DocTh>
            </tr>
          </thead>
          <tbody>
            <tr>
              <DocTd>OpenAI</DocTd>
              <DocTd>Yes</DocTd>
              <DocTd><DocCode>https://api.openai.com/v1</DocCode></DocTd>
            </tr>
            <tr>
              <DocTd>Anthropic</DocTd>
              <DocTd>Yes</DocTd>
              <DocTd><DocCode>https://api.anthropic.com</DocCode></DocTd>
            </tr>
            <tr>
              <DocTd>Google AI</DocTd>
              <DocTd>Yes</DocTd>
              <DocTd><DocCode>https://generativelanguage.googleapis.com</DocCode></DocTd>
            </tr>
            <tr>
              <DocTd>Ollama</DocTd>
              <DocTd>No</DocTd>
              <DocTd><DocCode>http://localhost:11434</DocCode></DocTd>
            </tr>
            <tr>
              <DocTd>LM Studio</DocTd>
              <DocTd>No</DocTd>
              <DocTd><DocCode>http://localhost:1234/v1</DocCode></DocTd>
            </tr>
          </tbody>
        </DocTable>

        <DocH3>Adding API Keys</DocH3>
        <DocOl>
          <DocLi>Navigate to the <DocStrong>Providers</DocStrong> page</DocLi>
          <DocLi>Select the provider you want to configure</DocLi>
          <DocLi>Click <DocStrong>Add Key</DocStrong> button</DocLi>
          <DocLi>Enter an alias (e.g., "Primary", "Backup") and your API key</DocLi>
          <DocLi>The key will be encrypted and stored securely</DocLi>
        </DocOl>

        <DocH3>Configuring Local Providers (Ollama/LM Studio)</DocH3>
        <DocP>For local providers like Ollama and LM Studio, you need to configure the correct endpoint:</DocP>
        <DocUl>
          <DocLi>
            <DocStrong>Running locally (without Docker)</DocStrong>: Use localhost
            <div className="ml-4 mt-1 text-sm">
              - Ollama: <DocCode>http://localhost:11434</DocCode><br />
              - LM Studio: <DocCode>http://localhost:1234/v1</DocCode>
            </div>
          </DocLi>
          <DocLi>
            <DocStrong>Running in Docker</DocStrong>: Use host.docker.internal
            <div className="ml-4 mt-1 text-sm">
              - Ollama: <DocCode>http://host.docker.internal:11434</DocCode><br />
              - LM Studio: <DocCode>http://host.docker.internal:1234/v1</DocCode>
            </div>
          </DocLi>
        </DocUl>
        <DocP>
          You can edit the endpoint URL in the Providers page by clicking the Edit button 
          in the Local Provider card.
        </DocP>

        <DocH3>Provider Priority and Weights</DocH3>
        <DocP>The router uses priority and weight to determine which provider to use:</DocP>
        <DocUl>
          <DocLi><DocStrong>Priority</DocStrong> (1-10): Lower numbers = higher priority. The router tries providers in priority order.</DocLi>
          <DocLi><DocStrong>Weight</DocStrong> (0-1): For providers with the same priority, weight determines the probability of selection.</DocLi>
        </DocUl>
      </div>
    ),
  },
  {
    id: 'proxy-config',
    title: 'Proxy Configuration',
    content: (
      <div>
        <DocH3>When to Use Proxies</DocH3>
        <DocP>Proxies are useful when:</DocP>
        <DocUl>
          <DocLi>Your network requires a proxy to access external APIs</DocLi>
          <DocLi>You need to route traffic through specific regions</DocLi>
          <DocLi>You want to add an additional layer of network control</DocLi>
        </DocUl>

        <DocH3>Adding a Proxy</DocH3>
        <DocOl>
          <DocLi>Navigate to the <DocStrong>Proxies</DocStrong> page</DocLi>
          <DocLi>Click <DocStrong>Add Proxy</DocStrong></DocLi>
          <DocLi>Enter the proxy URL (e.g., <DocCode>http://proxy.example.com:8080</DocCode> or <DocCode>socks5://proxy.example.com:1080</DocCode>)</DocLi>
          <DocLi>Select the proxy type (HTTP or SOCKS5)</DocLi>
          <DocLi>Optionally specify a region for geo-based routing</DocLi>
        </DocOl>

        <DocH3>Enabling Proxy for a Provider</DocH3>
        <DocOl>
          <DocLi>Go to the <DocStrong>Providers</DocStrong> page</DocLi>
          <DocLi>Select the provider</DocLi>
          <DocLi>Toggle the <DocStrong>Use Proxy</DocStrong> switch to enable</DocLi>
        </DocOl>
        <DocP>
          When enabled, all requests to that provider will be routed through the first available active proxy.
        </DocP>

        <DocH3>Proxy Health Monitoring</DocH3>
        <DocP>
          The platform automatically monitors proxy health. You can view proxy status in the 
          <DocStrong> Health</DocStrong> page under the Proxies tab. Unhealthy proxies will be 
          automatically skipped when routing requests.
        </DocP>
      </div>
    ),
  },
  {
    id: 'api-usage',
    title: 'API Usage',
    content: (
      <div>
        <DocH3>Chat Completions API</DocH3>
        <DocP>The router exposes an OpenAI-compatible API endpoint for chat completions:</DocP>
        <DocPre>{`POST /api/v1/chat/completions

Headers:
  Content-Type: application/json
  Authorization: Bearer <your-router-api-key>

Body:
{
  "model": "gpt-4",
  "messages": [
    {"role": "user", "content": "Hello!"}
  ],
  "stream": false
}`}</DocPre>

        <DocH3>Streaming Responses</DocH3>
        <DocP>To enable streaming, set <DocCode>stream: true</DocCode> in your request:</DocP>
        <DocPre>{`{
  "model": "gpt-4",
  "messages": [...],
  "stream": true
}`}</DocPre>
        <DocP>The response will be sent as Server-Sent Events (SSE).</DocP>

        <DocH3>Model Routing</DocH3>
        <DocP>The router automatically maps model names to the appropriate provider:</DocP>
        <DocUl>
          <DocLi><DocCode>gpt-*</DocCode> models route to OpenAI</DocLi>
          <DocLi><DocCode>claude-*</DocCode> models route to Anthropic</DocLi>
          <DocLi><DocCode>gemini-*</DocCode> models route to Google</DocLi>
          <DocLi>Other models check Ollama/LM Studio first</DocLi>
        </DocUl>

        <DocH3>Error Handling</DocH3>
        <DocP>
          If a provider fails, the router automatically retries with the next available provider 
          based on priority. The number of retries is configurable per provider.
        </DocP>
      </div>
    ),
  },
  {
    id: 'health-monitoring',
    title: 'Health Monitoring',
    content: (
      <div>
        <DocH3>Health Dashboard</DocH3>
        <DocP>The Health page provides real-time monitoring for:</DocP>
        <DocUl>
          <DocLi><DocStrong>Providers</DocStrong> - Check if LLM providers are accessible</DocLi>
          <DocLi><DocStrong>API Keys</DocStrong> - Verify API keys are valid and working</DocLi>
          <DocLi><DocStrong>Proxies</DocStrong> - Monitor proxy connectivity</DocLi>
          <DocLi><DocStrong>Alerts</DocStrong> - View and manage system alerts</DocLi>
        </DocUl>

        <DocH3>Health Check Endpoints</DocH3>
        <DocP>Each provider type uses a different health check endpoint:</DocP>
        <DocUl>
          <DocLi><DocStrong>OpenAI/LM Studio</DocStrong>: GET /models</DocLi>
          <DocLi><DocStrong>Ollama</DocStrong>: GET /api/tags</DocLi>
          <DocLi><DocStrong>Anthropic</DocStrong>: GET /v1/messages (with minimal payload)</DocLi>
        </DocUl>

        <DocH3>Success Rate</DocH3>
        <DocP>
          The success rate is calculated from the last 10 health checks. A success rate 
          below 80% may indicate connectivity issues that need investigation.
        </DocP>

        <DocH3>Alerts</DocH3>
        <DocP>The system generates alerts when:</DocP>
        <DocUl>
          <DocLi>A provider health check fails</DocLi>
          <DocLi>An API key becomes invalid</DocLi>
          <DocLi>A proxy connection fails</DocLi>
          <DocLi>Usage limits are approaching</DocLi>
        </DocUl>
        <DocP>Alerts can be acknowledged and resolved from the Health page.</DocP>
      </div>
    ),
  },
  {
    id: 'environment-variables',
    title: 'Environment Variables',
    content: (
      <div>
        <DocH3>Server Configuration</DocH3>
        <DocP>The server can be configured using environment variables or a .env file:</DocP>
        <DocTable>
          <thead>
            <tr>
              <DocTh>Variable</DocTh>
              <DocTh>Description</DocTh>
              <DocTh>Default</DocTh>
            </tr>
          </thead>
          <tbody>
            <tr>
              <DocTd><DocCode>PORT</DocCode></DocTd>
              <DocTd>Server port</DocTd>
              <DocTd>8080</DocTd>
            </tr>
            <tr>
              <DocTd><DocCode>DATABASE_URL</DocCode></DocTd>
              <DocTd>PostgreSQL connection string</DocTd>
              <DocTd>-</DocTd>
            </tr>
            <tr>
              <DocTd><DocCode>REDIS_URL</DocCode></DocTd>
              <DocTd>Redis connection string</DocTd>
              <DocTd>-</DocTd>
            </tr>
            <tr>
              <DocTd><DocCode>JWT_SECRET</DocCode></DocTd>
              <DocTd>Secret key for JWT tokens</DocTd>
              <DocTd>-</DocTd>
            </tr>
            <tr>
              <DocTd><DocCode>ENCRYPTION_KEY</DocCode></DocTd>
              <DocTd>Key for encrypting API keys (32 bytes)</DocTd>
              <DocTd>-</DocTd>
            </tr>
            <tr>
              <DocTd><DocCode>LOG_LEVEL</DocCode></DocTd>
              <DocTd>Logging level (debug, info, warn, error)</DocTd>
              <DocTd>info</DocTd>
            </tr>
          </tbody>
        </DocTable>

        <DocH3>Example .env File</DocH3>
        <DocPre>{`PORT=8080
DATABASE_URL=postgres://user:pass@localhost:5432/llmrouter?sslmode=disable
REDIS_URL=redis://localhost:6379
JWT_SECRET=your-secret-key-here
ENCRYPTION_KEY=your-32-byte-encryption-key-here
LOG_LEVEL=info`}</DocPre>

        <DocH3>Docker Compose Override</DocH3>
        <DocP>
          For local development, you can create a <DocCode>docker-compose.override.yml</DocCode> file 
          to customize settings without modifying the main compose file.
        </DocP>
      </div>
    ),
  },
  {
    id: 'troubleshooting',
    title: 'Troubleshooting',
    content: (
      <div>
        <DocH3>Common Issues</DocH3>

        <DocH4>Cannot connect to Ollama/LM Studio from Docker</DocH4>
        <DocP>
          When running in Docker, use <DocCode>host.docker.internal</DocCode> instead of <DocCode>localhost</DocCode>:
        </DocP>
        <DocUl>
          <DocLi>Ollama: <DocCode>http://host.docker.internal:11434</DocCode></DocLi>
          <DocLi>LM Studio: <DocCode>http://host.docker.internal:1234/v1</DocCode></DocLi>
        </DocUl>

        <DocH4>API Key validation fails</DocH4>
        <DocP>Check the following:</DocP>
        <DocUl>
          <DocLi>Ensure the API key is correct and has not expired</DocLi>
          <DocLi>Verify the provider is enabled in the Providers page</DocLi>
          <DocLi>Check if proxy is required and properly configured</DocLi>
          <DocLi>Review the Health page for detailed error messages</DocLi>
        </DocUl>

        <DocH4>High latency on requests</DocH4>
        <DocP>Possible causes:</DocP>
        <DocUl>
          <DocLi>Network issues between your server and the provider</DocLi>
          <DocLi>Proxy adding overhead - try disabling proxy if not needed</DocLi>
          <DocLi>Provider rate limiting - check your usage against limits</DocLi>
        </DocUl>

        <DocH4>Database connection errors</DocH4>
        <DocP>Ensure PostgreSQL is running and accessible:</DocP>
        <DocPre>{`# Check if PostgreSQL container is running
docker compose ps

# View PostgreSQL logs
docker compose logs postgres

# Test connection
docker compose exec postgres psql -U postgres -d llmrouter -c "SELECT 1"`}</DocPre>

        <DocH4>Redis connection errors</DocH4>
        <DocP>Similar to PostgreSQL, verify Redis is running:</DocP>
        <DocPre>{`# Check Redis container
docker compose logs redis

# Test connection
docker compose exec redis redis-cli ping`}</DocPre>

        <DocH3>Getting Help</DocH3>
        <DocP>If you encounter issues not covered here:</DocP>
        <DocUl>
          <DocLi>Check the server logs: <DocCode>docker compose logs server</DocCode></DocLi>
          <DocLi>Enable debug logging by setting <DocCode>LOG_LEVEL=debug</DocCode></DocLi>
          <DocLi>Open an issue on the <a href="https://github.com/Veritas-Calculus/llm-router-platform/issues" className="text-apple-blue hover:underline" target="_blank" rel="noopener noreferrer">GitHub repository</a> with relevant logs and configuration</DocLi>
        </DocUl>
      </div>
    ),
  },
];

function DocsPage() {
  const [activeSection, setActiveSection] = useState(sections[0].id);

  const currentSection = sections.find((s) => s.id === activeSection) || sections[0];

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-apple-gray-900">Documentation</h1>
        <p className="text-apple-gray-500 mt-1">Learn how to use and configure LLM Router</p>
      </div>

      <div className="flex gap-8">
        {/* Sidebar Navigation */}
        <nav className="w-64 shrink-0">
          <div className="sticky top-8 space-y-1 bg-white rounded-apple-lg p-3 shadow-apple">
            {sections.map((section) => (
              <button
                key={section.id}
                onClick={() => setActiveSection(section.id)}
                className={`w-full flex items-center justify-between px-3 py-2 rounded-apple text-left text-sm transition-colors ${
                  activeSection === section.id
                    ? 'bg-apple-blue text-white'
                    : 'text-apple-gray-600 hover:bg-apple-gray-100'
                }`}
              >
                <span className="font-medium">{section.title}</span>
                <ChevronRightIcon className={`w-4 h-4 ${activeSection === section.id ? 'text-white' : 'text-apple-gray-400'}`} />
              </button>
            ))}
          </div>
        </nav>

        {/* Content */}
        <motion.div
          key={activeSection}
          initial={{ opacity: 0, x: 10 }}
          animate={{ opacity: 1, x: 0 }}
          transition={{ duration: 0.2 }}
          className="flex-1 min-w-0"
        >
          <div className="card">
            <h2 className="text-xl font-semibold text-apple-gray-900 mb-6 pb-4 border-b border-apple-gray-100">
              {currentSection.title}
            </h2>
            <div>
              {currentSection.content}
            </div>
          </div>
        </motion.div>
      </div>
    </div>
  );
}

export default DocsPage;

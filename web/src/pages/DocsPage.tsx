import { motion } from 'framer-motion';
import { 
  BookOpenIcon, 
  CodeBracketIcon, 
  CommandLineIcon, 
  CubeIcon,
  ShieldCheckIcon
} from '@heroicons/react/24/outline';

function DocsPage() {
  const sections = [
    {
      title: 'Quick Start',
      icon: BookOpenIcon,
      content: `Welcome to the LLM Router Platform. To get started, you need to create an API Key. 
      Go to the "API Keys" page, create a new key, and use it to authenticate your requests.`
    },
    {
      title: 'API Reference',
      icon: CodeBracketIcon,
      content: `Our API is fully compatible with OpenAI's format. You can use any OpenAI SDK by changing the Base URL.
      
      Base URL: \`http://${window.location.host}/v1\`
      Authentication: \`Bearer YOUR_API_KEY\`
      
      Endpoints supported:
      - POST /chat/completions
      - POST /embeddings
      - POST /images/generations
      - GET /models`
    },
    {
      title: 'MCP (Model Context Protocol)',
      icon: CommandLineIcon,
      content: `Extend your LLMs with real-world capabilities. Our platform acts as an MCP Host. 
      When you configure MCP servers in the admin panel, tools are automatically injected into your chat requests.
      No client-side code changes required for basic tool usage.`
    },
    {
      title: 'Billing & Credits',
      icon: CubeIcon,
      content: `We use a hybrid billing system. 
      - **Subscriptions**: Choose a plan for higher RPM and monthly token quotas.
      - **Pay-as-you-go**: Top up your balance anytime. Requests will automatically deduct from your balance based on provider costs.`
    }
  ];

  return (
    <div className="max-w-4xl mx-auto space-y-8">
      <div>
        <h1 className="text-3xl font-bold text-apple-gray-900">Documentation</h1>
        <p className="text-apple-gray-500 mt-2">Learn how to integrate and use the platform</p>
      </div>

      <div className="grid grid-cols-1 gap-8">
        {sections.map((section, index) => (
          <motion.section
            key={section.title}
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: index * 0.1 }}
            className="bg-white rounded-3xl p-8 border border-apple-gray-200 shadow-sm"
          >
            <div className="flex items-center gap-4 mb-6">
              <div className="p-3 bg-blue-50 rounded-2xl text-apple-blue">
                <section.icon className="w-6 h-6" />
              </div>
              <h2 className="text-xl font-bold text-apple-gray-900">{section.title}</h2>
            </div>
            <div className="prose prose-apple max-w-none text-apple-gray-600 whitespace-pre-wrap">
              {section.content}
            </div>
          </motion.section>
        ))}
      </div>

      <div className="bg-apple-gray-900 rounded-3xl p-8 text-white flex items-center justify-between overflow-hidden relative">
        <div className="relative z-10">
          <h3 className="text-xl font-bold mb-2">Need Help?</h3>
          <p className="text-apple-gray-400">Our support team is here to help you with integration.</p>
          <button className="mt-6 px-6 py-2 bg-white text-apple-gray-900 rounded-xl font-semibold text-sm hover:bg-apple-gray-100 transition-colors">
            Contact Support
          </button>
        </div>
        <ShieldCheckIcon className="w-48 h-48 text-white/5 absolute -right-8 -bottom-8" />
      </div>
    </div>
  );
}

export default DocsPage;

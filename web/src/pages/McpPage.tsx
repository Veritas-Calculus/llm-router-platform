import { useState, useMemo } from 'react';
import { motion } from 'framer-motion';
import { 
  ServerIcon, 
  CommandLineIcon, 
  GlobeAltIcon, 
  PlusIcon, 
  TrashIcon, 
  ArrowPathIcon,
  CheckCircleIcon,
  XCircleIcon,
  ExclamationTriangleIcon,
  ChevronDownIcon,
  ChevronUpIcon
} from '@heroicons/react/24/outline';
import { useQuery, useMutation } from '@apollo/client/react';
import { MCP_SERVERS_QUERY, CREATE_MCP_SERVER, UPDATE_MCP_SERVER, DELETE_MCP_SERVER, REFRESH_MCP_TOOLS } from '@/lib/graphql/operations';
import type { McpServer } from '@/lib/types';
import toast from 'react-hot-toast';
import ConfirmModal from '@/components/ConfirmModal';
import { useTranslation } from '@/lib/i18n';

/* eslint-disable @typescript-eslint/no-explicit-any */

function McpPage() {
  const { t } = useTranslation();
  const { data, loading, refetch } = useQuery<any>(MCP_SERVERS_QUERY);
  const servers: McpServer[] = useMemo(() =>
    (data?.mcpServers || []).map((s: any) => ({
      id: s.id, name: s.name, type: s.type, command: s.command, args: s.args,
      env: s.env, url: s.url, is_active: s.isActive, status: s.status,
      last_error: s.lastError,
      tools: (s.tools || []).map((t: any) => ({ id: t.id, name: t.name, description: t.description, is_active: t.isActive })),
    })),
  [data]);
  const [createMut] = useMutation(CREATE_MCP_SERVER);
  const [updateMut] = useMutation(UPDATE_MCP_SERVER);
  const [deleteMut] = useMutation(DELETE_MCP_SERVER);
  const [refreshMut] = useMutation(REFRESH_MCP_TOOLS);

  const [isModalOpen, setIsModalOpen] = useState(false);
  const [isDeleteModalOpen, setIsModalDeleteOpen] = useState(false);
  const [selectedServer, setSelectedServer] = useState<McpServer | null>(null);
  const [expandedServers, setExpandedServers] = useState<Record<string, boolean>>({});

  const [formData, setFormData] = useState<Partial<McpServer>>({
    name: '',
    type: 'stdio',
    command: '',
    args: [],
    env: {},
    url: '',
    is_active: true
  });

  const handleOpenModal = (server?: McpServer) => {
    if (server) {
      setSelectedServer(server);
      setFormData({
        name: server.name,
        type: server.type,
        command: server.command,
        args: server.args || [],
        env: server.env || {},
        url: server.url,
        is_active: server.is_active
      });
    } else {
      setSelectedServer(null);
      setFormData({
        name: '',
        type: 'stdio',
        command: '',
        args: [],
        env: {},
        url: '',
        is_active: true
      });
    }
    setIsModalOpen(true);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const input = {
        name: formData.name, type: formData.type, command: formData.command,
        args: formData.args, url: formData.url, isActive: formData.is_active,
      };
      if (selectedServer) {
        await updateMut({ variables: { id: selectedServer.id, input } });
        toast.success('MCP server updated');
      } else {
        await createMut({ variables: { input } });
        toast.success('MCP server created');
      }
      setIsModalOpen(false);
      refetch();
    } catch {
      toast.error('Failed to save MCP server');
    }
  };

  const handleDelete = async () => {
    if (!selectedServer) return;
    try {
      await deleteMut({ variables: { id: selectedServer.id } });
      toast.success('MCP server deleted');
      setIsModalDeleteOpen(false);
      refetch();
    } catch {
      toast.error('Failed to delete MCP server');
    }
  };

  const handleRefreshTools = async (id: string) => {
    try {
      await refreshMut({ variables: { id } });
      toast.success('Tools refreshed');
      refetch();
    } catch {
      toast.error('Failed to refresh tools');
    }
  };

  const toggleExpand = (id: string) => {
    setExpandedServers(prev => ({
      ...prev,
      [id]: !prev[id]
    }));
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'connected':
        return (
          <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
            <CheckCircleIcon className="w-3 h-3 mr-1" />
            Connected
          </span>
        );
      case 'error':
        return (
          <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800">
            <XCircleIcon className="w-3 h-3 mr-1" />
            Error
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800">
            <ArrowPathIcon className="w-3 h-3 mr-1 animate-spin" />
            Disconnected
          </span>
        );
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-semibold text-apple-gray-900">MCP Servers</h1>
          <p className="text-apple-gray-500">Manage Model Context Protocol servers and tools</p>
        </div>
        <button
          onClick={() => handleOpenModal()}
          className="apple-button-primary flex items-center"
        >
          <PlusIcon className="w-5 h-5 mr-2" />
          Add Server
        </button>
      </div>

      {loading ? (
        <div className="flex justify-center py-12">
          <ArrowPathIcon className="w-8 h-8 text-apple-blue animate-spin" />
        </div>
      ) : servers.length === 0 ? (
        <div className="bg-white rounded-apple border border-apple-gray-200 p-12 text-center">
          <ServerIcon className="w-12 h-12 text-apple-gray-300 mx-auto mb-4" />
          <h3 className="text-lg font-medium text-apple-gray-900">No MCP Servers</h3>
          <p className="text-apple-gray-500 max-w-sm mx-auto mt-2">
            Add your first MCP server to extend the capabilities of your LLM models.
          </p>
          <button
            onClick={() => handleOpenModal()}
            className="mt-6 apple-button-secondary inline-flex items-center"
          >
            <PlusIcon className="w-5 h-5 mr-2" />
            Add Server
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-6">
          {servers.map((server) => (
            <motion.div
              key={server.id}
              layout
              className="bg-white rounded-apple border border-apple-gray-200 overflow-hidden shadow-sm"
            >
              <div className="p-6">
                <div className="flex items-start justify-between">
                  <div className="flex items-center">
                    <div className={`p-3 rounded-xl mr-4 ${server.type === 'stdio' ? 'bg-purple-50 text-purple-600' : 'bg-blue-50 text-apple-blue'}`}>
                      {server.type === 'stdio' ? (
                        <CommandLineIcon className="w-6 h-6" />
                      ) : (
                        <GlobeAltIcon className="w-6 h-6" />
                      )}
                    </div>
                    <div>
                      <h3 className="text-lg font-semibold text-apple-gray-900 flex items-center">
                        {server.name}
                        <span className="ml-3">{getStatusBadge(server.status)}</span>
                      </h3>
                      <p className="text-sm text-apple-gray-500 font-mono mt-1">
                        {server.type === 'stdio' ? server.command : server.url}
                      </p>
                    </div>
                  </div>
                  <div className="flex space-x-2">
                    <button
                      onClick={() => handleRefreshTools(server.id)}
                      className="p-2 text-apple-gray-400 hover:text-apple-blue transition-colors"
                      title={t('mcp.refresh_tools')}
                    >
                      <ArrowPathIcon className="w-5 h-5" />
                    </button>
                    <button
                      onClick={() => handleOpenModal(server)}
                      className="p-2 text-apple-gray-400 hover:text-apple-gray-600 transition-colors"
                      title={t('common.edit')}
                    >
                      <PlusIcon className="w-5 h-5 rotate-45" />
                    </button>
                    <button
                      onClick={() => {
                        setSelectedServer(server);
                        setIsModalDeleteOpen(true);
                      }}
                      className="p-2 text-apple-gray-400 hover:text-red-500 transition-colors"
                      title={t('common.delete')}
                    >
                      <TrashIcon className="w-5 h-5" />
                    </button>
                  </div>
                </div>

                {server.last_error && (
                  <div className="mt-4 p-3 bg-red-50 rounded-lg flex items-start border border-red-100">
                    <ExclamationTriangleIcon className="w-5 h-5 text-red-500 mr-2 shrink-0 mt-0.5" />
                    <p className="text-sm text-red-700">{server.last_error}</p>
                  </div>
                )}

                <div className="mt-6 border-t border-apple-gray-100 pt-4">
                  <button
                    onClick={() => toggleExpand(server.id)}
                    className="flex items-center text-sm font-medium text-apple-gray-700 hover:text-apple-gray-900 transition-colors"
                  >
                    {expandedServers[server.id] ? (
                      <>
                        <ChevronUpIcon className="w-4 h-4 mr-1" />
                        Hide Tools ({server.tools?.length || 0})
                      </>
                    ) : (
                      <>
                        <ChevronDownIcon className="w-4 h-4 mr-1" />
                        Show Tools ({server.tools?.length || 0})
                      </>
                    )}
                  </button>

                  {expandedServers[server.id] && (
                    <motion.div
                      initial={{ height: 0, opacity: 0 }}
                      animate={{ height: 'auto', opacity: 1 }}
                      className="mt-4 space-y-3"
                    >
                      {server.tools && server.tools.length > 0 ? (
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                          {server.tools.map((tool) => (
                            <div key={tool.id} className="p-3 bg-apple-gray-50 rounded-xl border border-apple-gray-100">
                              <div className="flex justify-between">
                                <h4 className="font-semibold text-apple-gray-900 text-sm">{tool.name}</h4>
                                <span className={`w-2 h-2 rounded-full mt-1.5 ${tool.is_active ? 'bg-green-500' : 'bg-apple-gray-300'}`} />
                              </div>
                              <p className="text-xs text-apple-gray-500 mt-1 line-clamp-2">{tool.description}</p>
                            </div>
                          ))}
                        </div>
                      ) : (
                        <p className="text-sm text-apple-gray-400 italic py-2">No tools discovered</p>
                      )}
                    </motion.div>
                  )}
                </div>
              </div>
            </motion.div>
          ))}
        </div>
      )}

      {/* Create/Edit Modal */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/40 backdrop-blur-sm">
          <motion.div
            initial={{ scale: 0.95, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            className="bg-white rounded-2xl shadow-2xl w-full max-w-lg overflow-hidden"
          >
            <div className="px-6 py-4 border-b border-apple-gray-100 flex justify-between items-center">
              <h3 className="text-lg font-semibold text-apple-gray-900">
                {selectedServer ? 'Edit MCP Server' : 'Add MCP Server'}
              </h3>
              <button onClick={() => setIsModalOpen(false)} className="text-apple-gray-400 hover:text-apple-gray-600">
                <PlusIcon className="w-6 h-6 rotate-45" />
              </button>
            </div>
            <form onSubmit={handleSubmit} className="p-6 space-y-4">
              <div>
                <label className="block text-sm font-medium text-apple-gray-700 mb-1">Server Name</label>
                <input
                  type="text"
                  required
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  className="apple-input w-full"
                  placeholder="e.g. google-search"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-apple-gray-700 mb-1">Transport Type</label>
                <select
                  value={formData.type}
                  onChange={(e) => setFormData({ ...formData, type: e.target.value as 'stdio' | 'sse' })}
                  className="apple-input w-full"
                >
                  <option value="stdio">Stdio (Local Process)</option>
                  <option value="sse">SSE (HTTP/Remote)</option>
                </select>
              </div>

              {formData.type === 'stdio' ? (
                <>
                  <div>
                    <label className="block text-sm font-medium text-apple-gray-700 mb-1">Command</label>
                    <input
                      type="text"
                      required
                      value={formData.command}
                      onChange={(e) => setFormData({ ...formData, command: e.target.value })}
                      className="apple-input w-full font-mono text-sm"
                      placeholder="e.g. npx"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-apple-gray-700 mb-1">Arguments (comma separated)</label>
                    <input
                      type="text"
                      value={formData.args?.join(', ')}
                      onChange={(e) => setFormData({ ...formData, args: e.target.value.split(',').map(s => s.trim()).filter(s => s) })}
                      className="apple-input w-full font-mono text-sm"
                      placeholder="e.g. -y, @modelcontextprotocol/server-google-search"
                    />
                  </div>
                </>
              ) : (
                <div>
                  <label className="block text-sm font-medium text-apple-gray-700 mb-1">Server URL</label>
                  <input
                    type="url"
                    required
                    value={formData.url}
                    onChange={(e) => setFormData({ ...formData, url: e.target.value })}
                    className="apple-input w-full font-mono text-sm"
                    placeholder="https://mcp.example.com/sse"
                  />
                </div>
              )}

              <div className="flex items-center pt-2">
                <input
                  type="checkbox"
                  id="is_active"
                  checked={formData.is_active}
                  onChange={(e) => setFormData({ ...formData, is_active: e.target.checked })}
                  className="rounded text-apple-blue focus:ring-apple-blue mr-2"
                />
                <label htmlFor="is_active" className="text-sm text-apple-gray-700">Server is active</label>
              </div>

              <div className="mt-8 flex space-x-3">
                <button
                  type="button"
                  onClick={() => setIsModalOpen(false)}
                  className="apple-button-secondary flex-1"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="apple-button-primary flex-1"
                >
                  Save Server
                </button>
              </div>
            </form>
          </motion.div>
        </div>
      )}

      <ConfirmModal
        isOpen={isDeleteModalOpen}
        onCancel={() => setIsModalDeleteOpen(false)}
        onConfirm={handleDelete}
        title={t('mcp.delete_confirm')}
        message={`Are you sure you want to delete "${selectedServer?.name}"? This will also remove all its discovered tools.`}
        confirmText={t('common.delete')}
        confirmColor="red"
      />
    </div>
  );
}

export default McpPage;

import { useState, useMemo, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { useQuery, useMutation } from '@apollo/client/react';
import { 
  GET_WEBHOOKS, CREATE_WEBHOOK_ENDPOINT, UPDATE_WEBHOOK_ENDPOINT, 
  DELETE_WEBHOOK_ENDPOINT, TEST_WEBHOOK_ENDPOINT, GET_WEBHOOK_DELIVERIES
} from '@/lib/graphql/operations/webhooks';
import { MY_ORGANIZATIONS, MY_PROJECTS } from '@/lib/graphql/operations';
import { 
  PlusIcon, TrashIcon, PencilSquareIcon, BoltIcon, 
  CheckCircleIcon, XCircleIcon, ClockIcon 
} from '@heroicons/react/24/outline';
import toast from 'react-hot-toast';
import type { Organization, Project } from '@/lib/types';

/* eslint-disable @typescript-eslint/no-explicit-any */

export default function WebhooksPage() {
  // Organization state
  const { data: orgData } = useQuery<any>(MY_ORGANIZATIONS);
  const orgs: Organization[] = useMemo(() => orgData?.myOrganizations || [], [orgData]);
  const [selectedOrgId, setSelectedOrgId] = useState<string>('');

  useEffect(() => {
    if (orgs.length > 0 && !selectedOrgId) {
      setSelectedOrgId(orgs[0].id);
    }
  }, [orgs, selectedOrgId]);

  // Project state
  const { data: projData } = useQuery<any>(MY_PROJECTS, {
    variables: { orgId: selectedOrgId },
    skip: !selectedOrgId,
  });
  const projects: Project[] = useMemo(() => projData?.myProjects || [], [projData]);
  const [selectedProjectId, setSelectedProjectId] = useState<string>('');

  useEffect(() => {
    if (projects.length > 0) {
      if (!selectedProjectId || !projects.find(p => p.id === selectedProjectId)) {
        setSelectedProjectId(projects[0].id);
      }
    } else if (projects.length === 0 && selectedProjectId) {
      setSelectedProjectId('');
    }
  }, [projects, selectedProjectId]);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingWebhook, setEditingWebhook] = useState<any>(null);
  const [selectedEndpointId, setSelectedEndpointId] = useState<string | null>(null);

  const { data, loading, refetch } = useQuery<any>(GET_WEBHOOKS, {
    variables: { projectId: selectedProjectId },
    skip: !selectedProjectId,
  });

  const { data: deliveriesData, loading: deliveriesLoading } = useQuery<any>(GET_WEBHOOK_DELIVERIES, {
    variables: { endpointId: selectedEndpointId, limit: 50 },
    skip: !selectedEndpointId,
    pollInterval: 5000, // Refresh automatically when viewing
  });

  const [createWebhook] = useMutation(CREATE_WEBHOOK_ENDPOINT, {
    onCompleted: (data: any) => {
      toast.success('Webhook created successfully!');
      // Display secret once
      if (data.createWebhookEndpoint.secret) {
        toast((_t) => (
          <div className="flex flex-col">
            <span className="font-medium text-amber-500">Important: Save this Secret</span>
            <span className="text-xs text-gray-400 mt-1 break-all bg-gray-900/50 p-2 rounded border border-gray-700">
              {data.createWebhookEndpoint.secret}
            </span>
            <span className="text-xs mt-1">This will not be shown again.</span>
          </div>
        ), { duration: 10000 });
      }
      refetch();
      setIsModalOpen(false);
    },
    onError: (error: any) => toast.error(error.message)
  });

  const [updateWebhook] = useMutation(UPDATE_WEBHOOK_ENDPOINT, {
    onCompleted: () => {
      toast.success('Webhook updated successfully!');
      refetch();
      setIsModalOpen(false);
      setEditingWebhook(null);
    },
    onError: (error: any) => toast.error(error.message)
  });

  const [deleteWebhook] = useMutation(DELETE_WEBHOOK_ENDPOINT, {
    onCompleted: () => {
      toast.success('Webhook deleted');
      if (selectedEndpointId === editingWebhook?.id) setSelectedEndpointId(null);
      refetch();
    },
    onError: (error: any) => toast.error(error.message)
  });

  const [testWebhook] = useMutation(TEST_WEBHOOK_ENDPOINT, {
    onCompleted: () => {
      toast.success('Test ping event triggered! Please check deliveries below soon.');
    },
    onError: (error: any) => toast.error(error.message)
  });

  // Modal State
  interface WebhookFormData {
    url: string;
    description: string;
    events: string[];
    isActive: boolean;
  }
  
  const [formData, setFormData] = useState<WebhookFormData>({
    url: '',
    description: '',
    events: ['ping', 'payment.succeeded'],
    isActive: true
  });

  const handleOpenModal = (webhook: any = null) => {
    if (webhook) {
      setEditingWebhook(webhook);
      setFormData({
        url: webhook.url,
        description: webhook.description || '',
        events: webhook.events || [],
        isActive: webhook.isActive
      });
    } else {
      setEditingWebhook(null);
      setFormData({
        url: '',
        description: '',
        events: ['ping'],
        isActive: true
      });
    }
    setIsModalOpen(true);
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedProjectId) return;
    
    if (editingWebhook) {
      updateWebhook({
        variables: {
          id: editingWebhook.id,
          input: {
            url: formData.url,
            description: formData.description,
            events: formData.events,
            isActive: formData.isActive
          }
        }
      });
    } else {
      createWebhook({
        variables: {
          input: {
            projectId: selectedProjectId,
            url: formData.url,
            description: formData.description,
            events: formData.events
          }
        }
      });
    }
  };

  if (!selectedProjectId) {
    return (
      <div className="space-y-6 max-w-7xl mx-auto">
        <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
          <div>
            <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">Webhooks</h1>
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
              Configure external endpoints to receive real-time notifications.
            </p>
          </div>
          <div className="flex items-center gap-2">
            <div className="flex flex-col gap-1">
              <label className="text-xs font-medium text-gray-500 dark:text-gray-400">Organization</label>
              <select
                title="Organization"
                value={selectedOrgId}
                onChange={(e) => setSelectedOrgId(e.target.value)}
                className="block w-48 rounded-xl border-gray-300 dark:border-white/10 bg-white dark:bg-white/5 py-2 pl-3 pr-10 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 text-gray-900 dark:text-white"
              >
                <option value="" disabled>Select Organization</option>
                {orgs.map((o) => (
                  <option key={o.id} value={o.id} className="text-gray-900 dark:bg-[#1A1A1A] dark:text-white">{o.name}</option>
                ))}
              </select>
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-xs font-medium text-gray-500 dark:text-gray-400">Project</label>
              <select
                title="Project"
                value={selectedProjectId}
                onChange={(e) => setSelectedProjectId(e.target.value)}
                className="block w-48 rounded-xl border-gray-300 dark:border-white/10 bg-white dark:bg-white/5 py-2 pl-3 pr-10 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 text-gray-900 dark:text-white"
              >
                <option value="" disabled>Select Project</option>
                {projects.map((p) => (
                  <option key={p.id} value={p.id} className="text-gray-900 dark:bg-[#1A1A1A] dark:text-white">{p.name}</option>
                ))}
              </select>
            </div>
          </div>
        </div>
        <div className="bg-white dark:bg-[#1A1A1A] rounded-2xl border border-gray-200 dark:border-white/10 shadow-sm p-12 text-center">
          <BoltIcon className="w-12 h-12 text-gray-300 dark:text-gray-600 mx-auto mb-4" />
          <h3 className="text-lg font-medium text-gray-700 dark:text-gray-300 mb-2">
            {projects.length === 0 ? 'No Projects Available' : 'Select a Project'}
          </h3>
          <p className="text-sm text-gray-500 dark:text-gray-400 max-w-md mx-auto">
            {projects.length === 0
              ? 'Create a project first to start configuring webhooks.'
              : 'Choose a project from the dropdown above to manage its webhook endpoints.'}
          </p>
        </div>
      </div>
    );
  }

  const parseJson = (str: string) => {
    try { return JSON.stringify(JSON.parse(str), null, 2); }
    catch { return str; }
  };

  return (
    <div className="space-y-6 max-w-7xl mx-auto">
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">Webhooks</h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Configure external endpoints to receive real-time notifications for structural events in {projects.find(p => p.id === selectedProjectId)?.name}.
          </p>
        </div>
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <div className="flex flex-col gap-1">
              <label className="text-xs font-medium text-gray-500 dark:text-gray-400">Organization</label>
              <select
                title="Organization"
                value={selectedOrgId}
                onChange={(e) => setSelectedOrgId(e.target.value)}
                className="block w-48 rounded-xl border-gray-300 dark:border-white/10 bg-white dark:bg-white/5 py-2 pl-3 pr-10 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 text-gray-900 dark:text-white"
              >
                <option value="" disabled>Select Organization</option>
                {orgs.map((o) => (
                  <option key={o.id} value={o.id} className="text-gray-900 dark:bg-[#1A1A1A] dark:text-white">{o.name}</option>
                ))}
              </select>
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-xs font-medium text-gray-500 dark:text-gray-400">Project</label>
              <select
                title="Project"
                value={selectedProjectId}
                onChange={(e) => setSelectedProjectId(e.target.value)}
                className="block w-48 rounded-xl border-gray-300 dark:border-white/10 bg-white dark:bg-white/5 py-2 pl-3 pr-10 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 text-gray-900 dark:text-white"
              >
                <option value="" disabled>Select Project</option>
                {projects.map((p) => (
                  <option key={p.id} value={p.id} className="text-gray-900 dark:bg-[#1A1A1A] dark:text-white">{p.name}</option>
                ))}
              </select>
            </div>
          </div>
          <button
            onClick={() => handleOpenModal()}
            className="inline-flex items-center gap-x-2 rounded-xl bg-indigo-600 px-4 py-2.5 text-sm font-medium text-white shadow hover:bg-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 transition-colors"
          >
            <PlusIcon className="-ml-0.5 h-5 w-5" aria-hidden="true" />
            Add Endpoint
          </button>
        </div>
      </div>

      <div className="bg-white dark:bg-[#1A1A1A] rounded-2xl border border-gray-200 dark:border-white/10 shadow-sm overflow-hidden">
        {loading ? (
          <div className="p-8 text-center text-gray-500 dark:text-gray-400">Loading webhooks...</div>
        ) : data?.webhooks?.length === 0 ? (
          <div className="p-12 text-center text-gray-500 dark:text-gray-400">
            No webhooks configured. Add your first webhook to receive events!
          </div>
        ) : (
          <table className="min-w-full divide-y divide-gray-200 dark:divide-white/10">
            <thead className="bg-gray-50 dark:bg-white/5">
              <tr>
                <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">URL & Description</th>
                <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Events</th>
                <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Status</th>
                <th scope="col" className="relative px-6 py-3"><span className="sr-only">Actions</span></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 dark:divide-white/10 bg-white dark:bg-transparent">
              {data?.webhooks?.map((webhook: any) => (
                <tr key={webhook.id}>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="flex flex-col">
                      <span className="text-sm font-medium text-gray-900 dark:text-white truncate max-w-xs">{webhook.url}</span>
                      <span className="text-xs text-gray-500 dark:text-gray-400 truncate max-w-xs mt-1">{webhook.description || 'No description'}</span>
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex flex-wrap gap-1">
                      {webhook.events.map((e: string) => (
                        <span key={e} className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 dark:bg-white/10 text-gray-800 dark:text-gray-300">
                          {e}
                        </span>
                      ))}
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                      webhook.isActive 
                        ? 'bg-green-100 text-green-800 dark:bg-green-400/10 dark:text-green-400' 
                        : 'bg-gray-100 text-gray-800 dark:bg-white/10 dark:text-gray-400'
                    }`}>
                      {webhook.isActive ? 'Active' : 'Disabled'}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                    <div className="flex items-center justify-end gap-3">
                      <button 
                        title="Test Ping"
                        onClick={() => {
                          setSelectedEndpointId(webhook.id);
                          testWebhook({ variables: { id: webhook.id } });
                        }}
                        className="text-indigo-600 hover:text-indigo-900 dark:text-indigo-400 dark:hover:text-indigo-300 transition-colors"
                      >
                        <BoltIcon className="w-5 h-5" />
                      </button>
                      <button 
                        title="View Deliveries"
                        onClick={() => setSelectedEndpointId(selectedEndpointId === webhook.id ? null : webhook.id)}
                        className={`transition-colors ${selectedEndpointId === webhook.id ? 'text-green-600 dark:text-green-400' : 'text-gray-400 hover:text-gray-600 dark:hover:text-gray-300'}`}
                      >
                        <ClockIcon className="w-5 h-5" />
                      </button>
                      <button 
                        title="Edit"
                        onClick={() => handleOpenModal(webhook)}
                        className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
                      >
                        <PencilSquareIcon className="w-5 h-5" />
                      </button>
                      <button 
                        title="Delete"
                        onClick={() => {
                          if (window.confirm('Are you sure you want to delete this webhook endpoint?')) {
                            deleteWebhook({ variables: { id: webhook.id } });
                          }
                        }}
                        className="text-red-600 hover:text-red-900 dark:text-red-500 dark:hover:text-red-400 transition-colors"
                      >
                        <TrashIcon className="w-5 h-5" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Deliveries View Panel */}
      <AnimatePresence>
        {selectedEndpointId && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 'auto' }}
            exit={{ opacity: 0, height: 0 }}
            className="bg-white dark:bg-[#1A1A1A] rounded-2xl border border-gray-200 dark:border-white/10 shadow-sm overflow-hidden flex flex-col"
          >
            <div className="px-6 py-4 border-b border-gray-200 dark:border-white/10 bg-gray-50 dark:bg-white/5 flex justify-between items-center">
              <h3 className="text-lg font-medium text-gray-900 dark:text-white">Recent Deliveries</h3>
              <button 
                onClick={() => setSelectedEndpointId(null)}
                className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
              >
                Close
              </button>
            </div>
            
            <div className="max-h-[600px] overflow-y-auto w-full p-6">
              {deliveriesLoading ? (
                <div className="text-center text-gray-500 py-4">Loading deliveries...</div>
              ) : deliveriesData?.webhookDeliveries?.length === 0 ? (
                <div className="text-center text-gray-500 py-8">No events have been dispatched to this endpoint yet. Try triggering a test ping!</div>
              ) : (
                <div className="space-y-4">
                  {deliveriesData?.webhookDeliveries?.map((d: any) => (
                    <div key={d.id} className="border border-gray-200 dark:border-white/10 rounded-xl overflow-hidden shadow-[0_1px_2px_rgba(0,0,0,0.05)] dark:shadow-none bg-white dark:bg-[#222222]">
                      {/* Delivery Header */}
                      <div className="bg-gray-50 dark:bg-white/5 px-4 py-3 flex items-center justify-between border-b border-gray-200 dark:border-white/10">
                        <div className="flex items-center gap-3">
                          {d.status === 'success' ? (
                            <CheckCircleIcon className="w-5 h-5 text-green-500" />
                          ) : d.status === 'pending' ? (
                            <ClockIcon className="w-5 h-5 text-amber-500" />
                          ) : (
                            <XCircleIcon className="w-5 h-5 text-red-500" />
                          )}
                          <div className="flex flex-col">
                            <span className="text-sm font-medium text-gray-900 dark:text-white">{d.eventType}</span>
                            <span className="text-xs text-gray-500 dark:text-gray-400">{new Date(d.createdAt).toLocaleString()}</span>
                          </div>
                        </div>
                        <div className="flex items-center gap-3">
                          <span className="text-xs font-mono text-gray-500">HTTP {d.statusCode || '---'}</span>
                          {d.retryCount > 0 && <span className="text-xs text-amber-500 font-medium">Retry {d.retryCount}</span>}
                        </div>
                      </div>
                      
                      {/* Delivery Details */}
                      <div className="p-4 grid grid-cols-1 md:grid-cols-2 gap-4 text-xs font-mono">
                        <div>
                          <p className="font-semibold text-gray-700 dark:text-gray-300 mb-2">Request Payload:</p>
                          <div className="p-3 bg-gray-100 dark:bg-black/50 rounded-lg overflow-x-auto border border-gray-200 dark:border-white/5">
                            <pre className="text-[11px] text-gray-800 dark:text-gray-300">{parseJson(d.payload)}</pre>
                          </div>
                        </div>
                        <div>
                          <p className="font-semibold text-gray-700 dark:text-gray-300 mb-2">Response / Error:</p>
                          <div className="p-3 bg-gray-100 dark:bg-black/50 rounded-lg overflow-x-auto min-h-[4rem] border border-gray-200 dark:border-white/5">
                            {d.errorMessage ? (
                              <pre className="text-[11px] text-red-600 dark:text-red-400 whitespace-pre-wrap">{d.errorMessage}</pre>
                            ) : (
                              <pre className="text-[11px] text-gray-800 dark:text-gray-300 break-all whitespace-pre-wrap">{d.responseBody || 'No response body'}</pre>
                            )}
                          </div>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Modal Overlay */}
      <AnimatePresence>
        {isModalOpen && (
          <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              onClick={() => setIsModalOpen(false)}
              className="absolute inset-0 bg-gray-500/75 dark:bg-black/80 backdrop-blur-sm transition-opacity"
            />
            
            <motion.div
              initial={{ opacity: 0, scale: 0.95, y: 20 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.95, y: 20 }}
              className="relative w-full max-w-lg bg-white dark:bg-[#1A1A1A] rounded-2xl shadow-xl overflow-hidden border border-gray-200 dark:border-white/10"
            >
              <div className="px-6 py-5 border-b border-gray-200 dark:border-white/10">
                <h3 className="text-xl font-semibold text-gray-900 dark:text-white">
                  {editingWebhook ? 'Edit Webhook' : 'Add Webhook Endpoint'}
                </h3>
              </div>
              
              <form onSubmit={handleSubmit} className="p-6 space-y-5">
                <div>
                  <label htmlFor="url" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Payload URL</label>
                  <input
                    type="url"
                    id="url"
                    required
                    value={formData.url}
                    onChange={(e) => setFormData({ ...formData, url: e.target.value })}
                    className="mt-2 block w-full rounded-xl border-gray-300 dark:border-white/10 bg-white dark:bg-white/5 px-4 py-2.5 text-gray-900 dark:text-white focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
                    placeholder="https://example.com/webhook"
                  />
                </div>
                
                <div>
                  <label htmlFor="description" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Description (Optional)</label>
                  <input
                    type="text"
                    id="description"
                    value={formData.description}
                    onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                    className="mt-2 block w-full rounded-xl border-gray-300 dark:border-white/10 bg-white dark:bg-white/5 px-4 py-2.5 text-gray-900 dark:text-white focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
                    placeholder="My staging environment webhook"
                  />
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Events to send</label>
                  <div className="space-y-2">
                    <label className="flex items-center gap-3 p-3 rounded-lg border border-gray-200 dark:border-white/10 bg-gray-50 dark:bg-white/5 cursor-pointer hover:bg-gray-100 dark:hover:bg-white/10 transition-colors">
                      <input 
                        type="checkbox" 
                        className="rounded border-gray-300 text-indigo-600 focus:ring-indigo-600 cursor-pointer h-4 w-4 bg-transparent"
                        checked={formData.events.includes('ping')}
                        onChange={(e) => {
                          const newEvents = e.target.checked 
                            ? [...formData.events, 'ping'] 
                            : formData.events.filter(ev => ev !== 'ping');
                          setFormData({ ...formData, events: newEvents });
                        }}
                      />
                      <span className="text-sm font-medium text-gray-900 dark:text-white">ping</span>
                      <span className="text-xs text-gray-500 ml-auto">System testing event</span>
                    </label>
                    <label className="flex items-center gap-3 p-3 rounded-lg border border-gray-200 dark:border-white/10 bg-gray-50 dark:bg-white/5 cursor-pointer hover:bg-gray-100 dark:hover:bg-white/10 transition-colors">
                      <input 
                        type="checkbox" 
                        className="rounded border-gray-300 text-indigo-600 focus:ring-indigo-600 cursor-pointer h-4 w-4 bg-transparent"
                        checked={formData.events.includes('payment.succeeded')}
                        onChange={(e) => {
                          const newEvents = e.target.checked 
                            ? [...formData.events, 'payment.succeeded'] 
                            : formData.events.filter(ev => ev !== 'payment.succeeded');
                          setFormData({ ...formData, events: newEvents });
                        }}
                      />
                      <span className="text-sm font-medium text-gray-900 dark:text-white">payment.succeeded</span>
                      <span className="text-xs text-gray-500 ml-auto">Account recharge verified</span>
                    </label>
                  </div>
                </div>

                <div className="flex items-center gap-3 pt-2">
                  <div className="flex h-6 items-center">
                    <input
                      id="isActive"
                      type="checkbox"
                      checked={formData.isActive}
                      onChange={(e) => setFormData({ ...formData, isActive: e.target.checked })}
                      className="h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-600 bg-transparent cursor-pointer"
                    />
                  </div>
                  <div className="text-sm">
                    <label htmlFor="isActive" className="font-medium text-gray-900 dark:text-white cursor-pointer">Active</label>
                    <p className="text-gray-500 dark:text-gray-400 text-xs mt-0.5">We will deliver events to this URL when active.</p>
                  </div>
                </div>

                <div className="pt-4 flex items-center justify-end gap-3 border-t border-gray-200 dark:border-white/10 mt-6">
                  <button
                    type="button"
                    onClick={() => setIsModalOpen(false)}
                    className="px-4 py-2.5 rounded-xl text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-white/10 transition-colors"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    className="px-4 py-2.5 rounded-xl text-sm font-medium text-white shadow bg-indigo-600 hover:bg-indigo-500 transition-colors focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
                  >
                    {editingWebhook ? 'Save Changes' : 'Create Webhook'}
                  </button>
                </div>
              </form>
            </motion.div>
          </div>
        )}
      </AnimatePresence>
    </div>
  );
}

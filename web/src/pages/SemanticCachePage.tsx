import { useState, useMemo, useEffect } from 'react';
import { useQuery, useMutation } from '@apollo/client/react';
import {
  GET_SEMANTIC_CACHES,
  GET_CACHE_STATS,
  CLEAR_SEMANTIC_CACHE,
  CLEAR_ALL_SEMANTIC_CACHES,
  CACHE_CONFIG_QUERY,
  UPDATE_CACHE_CONFIG,
} from '@/lib/graphql/operations/cache';
import toast from 'react-hot-toast';
import { format } from 'date-fns';
import { TrashIcon, BeakerIcon, Cog6ToothIcon, ChevronDownIcon } from '@heroicons/react/24/outline';
import { motion, AnimatePresence } from 'framer-motion';
import { useTranslation } from '@/lib/i18n';

/* eslint-disable @typescript-eslint/no-explicit-any */

// ─── Cache Config Panel ─────────────────────────────────────────────

function CacheConfigPanel() {
  const { t } = useTranslation();
  const { data, refetch } = useQuery<any>(CACHE_CONFIG_QUERY, { fetchPolicy: 'cache-and-network' });
  const [updateMut, { loading: saving }] = useMutation(UPDATE_CACHE_CONFIG);
  const [expanded, setExpanded] = useState(false);

  const [isEnabled, setIsEnabled] = useState(true);
  const [similarityThreshold, setSimilarityThreshold] = useState('0.05');
  const [defaultTtlMinutes, setDefaultTtlMinutes] = useState('1440');
  const [embeddingModel, setEmbeddingModel] = useState('text-embedding-3-small');
  const [maxCacheSize, setMaxCacheSize] = useState('10000');

  useEffect(() => {
    const cfg = data?.cacheConfig;
    if (cfg) {
      setIsEnabled(cfg.isEnabled);
      setSimilarityThreshold(String(cfg.similarityThreshold));
      setDefaultTtlMinutes(String(cfg.defaultTtlMinutes));
      setEmbeddingModel(cfg.embeddingModel);
      setMaxCacheSize(String(cfg.maxCacheSize));
    }
  }, [data]);

  const handleSave = async () => {
    try {
      await updateMut({
        variables: {
          input: {
            isEnabled,
            similarityThreshold: parseFloat(similarityThreshold) || 0.05,
            defaultTtlMinutes: parseInt(defaultTtlMinutes, 10) || 1440,
            embeddingModel,
            maxCacheSize: parseInt(maxCacheSize, 10) || 10000,
          },
        },
      });
      await refetch();
      toast.success('Cache configuration saved');
    } catch {
      toast.error('Failed to save config');
    }
  };

  return (
    <div className="bg-white rounded-xl shadow-apple-sm border border-apple-gray-100 overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full px-6 py-4 flex items-center justify-between hover:bg-apple-gray-50/50 transition-colors"
      >
        <div className="flex items-center gap-3">
          <div className="p-2 bg-purple-50 text-purple-600 rounded-lg">
            <Cog6ToothIcon className="w-5 h-5" />
          </div>
          <div className="text-left">
            <h2 className="text-base font-medium text-apple-gray-900">Cache Configuration</h2>
            <p className="text-xs text-apple-gray-500 mt-0.5">Similarity threshold, TTL, embedding model</p>
          </div>
        </div>
        <ChevronDownIcon
          className={`w-5 h-5 text-apple-gray-400 transition-transform ${expanded ? 'rotate-180' : ''}`}
        />
      </button>

      <AnimatePresence>
        {expanded && (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            className="overflow-hidden"
          >
            <div className="px-6 pb-6 pt-2 border-t border-apple-gray-100">
              <div className="flex items-center justify-between mb-5">
                <span className="text-sm font-medium text-apple-gray-700">Enable Semantic Cache</span>
                <label className="relative inline-flex items-center cursor-pointer">
                  <input
                    type="checkbox"
                    checked={isEnabled}
                    onChange={(e) => setIsEnabled(e.target.checked)}
                    className="sr-only peer"
                  />
                  <div className="w-11 h-6 bg-apple-gray-200 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-apple-blue" />
                </label>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
                <div>
                  <label className="label">Similarity Threshold</label>
                  <input
                    type="number"
                    value={similarityThreshold}
                    onChange={(e) => setSimilarityThreshold(e.target.value)}
                    className="input mt-1 w-full"
                    step="0.01"
                    min="0"
                    max="1"
                  />
                  <p className="text-xs text-apple-gray-400 mt-1">Cosine distance (lower = stricter)</p>
                </div>

                <div>
                  <label className="label">Default TTL (min)</label>
                  <input
                    type="number"
                    value={defaultTtlMinutes}
                    onChange={(e) => setDefaultTtlMinutes(e.target.value)}
                    className="input mt-1 w-full"
                    min="1"
                  />
                  <p className="text-xs text-apple-gray-400 mt-1">Cache expiry in minutes</p>
                </div>

                <div>
                  <label className="label">Embedding Model</label>
                  <select
                    value={embeddingModel}
                    onChange={(e) => setEmbeddingModel(e.target.value)}
                    className="input mt-1 w-full"
                  >
                    <option value="text-embedding-3-small">text-embedding-3-small</option>
                    <option value="text-embedding-3-large">text-embedding-3-large</option>
                    <option value="text-embedding-ada-002">text-embedding-ada-002</option>
                  </select>
                  <p className="text-xs text-apple-gray-400 mt-1">Vector embedding model</p>
                </div>

                <div>
                  <label className="label">Max Cache Size</label>
                  <input
                    type="number"
                    value={maxCacheSize}
                    onChange={(e) => setMaxCacheSize(e.target.value)}
                    className="input mt-1 w-full"
                    min="100"
                  />
                  <p className="text-xs text-apple-gray-400 mt-1">Maximum cache entries</p>
                </div>
              </div>

              <div className="flex justify-end mt-5">
                <button onClick={handleSave} className="btn btn-primary" disabled={saving}>
                  {saving ? 'Saving...' : 'Save Configuration'}
                </button>
              </div>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

// ─── Main Page ──────────────────────────────────────────────────────

function SemanticCachePage() {
  const { data: statsData, refetch: refetchStats } = useQuery<{ cacheStats: any }>(GET_CACHE_STATS);
  const { data: cachesData, loading, refetch: refetchCaches } = useQuery<{ semanticCaches: any[] }>(GET_SEMANTIC_CACHES, {
    variables: { limit: 100, offset: 0 },
  });

  const [clearCacheMut] = useMutation(CLEAR_SEMANTIC_CACHE);
  const [clearAllCachesMut] = useMutation(CLEAR_ALL_SEMANTIC_CACHES);

  const [isClearingAll, setIsClearingAll] = useState(false);

  const stats = statsData?.cacheStats || { totalCaches: 0, totalHits: 0 };
  const caches = useMemo(() => cachesData?.semanticCaches || [], [cachesData]);

  const handleClearCache = async (id: string) => {
    if (!window.confirm('Delete this cache entry?')) return;
    try {
      await clearCacheMut({ variables: { id } });
      toast.success('Cache entry deleted');
      refetchCaches();
      refetchStats();
    } catch {
      toast.error('Failed to delete cache');
    }
  };

  const handleClearAll = async () => {
    if (!window.confirm('Are you absolutely sure you want to clear ALL semantic caches?')) return;
    setIsClearingAll(true);
    try {
      await clearAllCachesMut();
      toast.success('All semantic caches cleared');
      refetchCaches();
      refetchStats();
    } catch {
      toast.error('Failed to clear all caches');
    } finally {
      setIsClearingAll(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-apple-gray-900">Semantic Cache</h1>
          <p className="text-apple-gray-500 mt-1">Manage and monitor vectorized prompt caching (pgvector)</p>
        </div>
        <button
          onClick={handleClearAll}
          disabled={isClearingAll || caches.length === 0}
          className="px-4 py-2 border border-red-300 text-red-600 rounded-lg text-sm font-medium hover:bg-red-50 disabled:opacity-50 transition-colors flex items-center gap-2"
        >
          <TrashIcon className="w-4 h-4" />
          {isClearingAll ? 'Clearing...' : 'Clear All Caches'}
        </button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div className="bg-white rounded-xl shadow-apple-sm p-6 border border-apple-gray-100 flex items-center gap-4">
          <div className="p-3 bg-blue-50 text-apple-blue rounded-lg">
            <BeakerIcon className="w-6 h-6" />
          </div>
          <div>
            <div className="text-sm font-medium text-apple-gray-500">Total Vector Caches</div>
            <div className="text-2xl font-semibold text-apple-gray-900">{stats.totalCaches}</div>
          </div>
        </div>
        <div className="bg-white rounded-xl shadow-apple-sm p-6 border border-apple-gray-100 flex items-center gap-4">
          <div className="p-3 bg-green-50 text-green-600 rounded-lg">
            <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
            </svg>
          </div>
          <div>
            <div className="text-sm font-medium text-apple-gray-500">Total Cache Hits</div>
            <div className="text-2xl font-semibold text-apple-gray-900">{stats.totalHits}</div>
            <div className="text-xs text-apple-gray-400 mt-0.5">Approx. ${(stats.totalHits * 0.005).toFixed(2)} saved</div>
          </div>
        </div>
      </div>

      {/* Cache Configuration Panel */}
      <CacheConfigPanel />

      <div className="bg-white rounded-xl shadow-apple-sm border border-apple-gray-100 overflow-hidden">
        <div className="px-6 py-4 border-b border-apple-gray-100">
          <h2 className="text-lg font-medium text-apple-gray-900">Recent Cache Entries</h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm whitespace-nowrap">
            <thead className="bg-apple-gray-50/50 text-apple-gray-500 font-medium">
              <tr>
                <th className="px-6 py-3 border-y border-apple-gray-100">Creation Date</th>
                <th className="px-6 py-3 border-y border-apple-gray-100">Provider</th>
                <th className="px-6 py-3 border-y border-apple-gray-100">Model</th>
                <th className="px-6 py-3 border-y border-apple-gray-100">Hash (SHA-256)</th>
                <th className="px-6 py-3 border-y border-apple-gray-100">Hits</th>
                <th className="px-6 py-3 border-y border-apple-gray-100 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-apple-gray-100">
              {caches.map((c: any) => (
                <tr key={c.id} className="hover:bg-apple-gray-50/50 transition-colors">
                  <td className="px-6 py-4 text-apple-gray-900">
                    {format(new Date(c.createdAt), 'MMM d, yyyy HH:mm:ss')}
                  </td>
                  <td className="px-6 py-4">
                    <span className="inline-flex items-center px-2 py-1 rounded-md bg-apple-gray-100 text-apple-gray-700 text-xs font-medium">
                      {c.provider}
                    </span>
                  </td>
                  <td className="px-6 py-4 text-apple-gray-900 font-medium">
                    {c.model}
                  </td>
                  <td className="px-6 py-4 text-apple-gray-500 font-mono text-xs">
                    {c.hash.substring(0, 16)}...
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-1.5">
                      <span className="text-apple-gray-900 font-medium">{c.hitCount}</span>
                      {c.hitCount > 0 && <span className="text-green-500 text-xs">+</span>}
                    </div>
                  </td>
                  <td className="px-6 py-4 text-right">
                    <button
                      onClick={() => handleClearCache(c.id)}
                      className="text-apple-gray-400 hover:text-red-500 transition-colors p-1"
                      title={t('cache.delete_entry')}
                    >
                      <TrashIcon className="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              ))}
              {caches.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-6 py-12 text-center text-apple-gray-500">
                    No semantics caches recorded yet.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

export default SemanticCachePage;


import {
  CheckCircleIcon,
  PlusIcon,
  ServerStackIcon,
  TrashIcon,
  XCircleIcon,
} from '@heroicons/react/24/outline';
import { ProxyPool } from '@/lib/types';

interface ProxyPoolDraft {
  name: string;
  description: string;
  strategy: string;
}

interface ProxyPoolsPanelProps {
  proxyPools: ProxyPool[];
  poolDraft: ProxyPoolDraft;
  creatingPool: boolean;
  updatingPoolId: string | null;
  deletingPoolId: string | null;
  onDraftChange: (draft: ProxyPoolDraft) => void;
  onCreatePool: () => void;
  onTogglePool: (pool: ProxyPool) => void;
  onDeletePool: (id: string) => void;
}

export default function ProxyPoolsPanel({
  proxyPools,
  poolDraft,
  creatingPool,
  updatingPoolId,
  deletingPoolId,
  onDraftChange,
  onCreatePool,
  onTogglePool,
  onDeletePool,
}: ProxyPoolsPanelProps) {
  const canCreate = Boolean(poolDraft.name.trim() && !creatingPool);

  return (
    <div className="card">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <div className="flex items-center gap-2">
            <ServerStackIcon className="w-5 h-5 text-apple-blue" />
            <h2 className="text-lg font-semibold text-apple-gray-900">Proxy Pools</h2>
          </div>
          <p className="text-sm text-apple-gray-500 mt-1">
            Group proxies before binding accounts to a stable egress set.
          </p>
        </div>
        <form
          className="grid grid-cols-1 md:grid-cols-[minmax(160px,1fr)_minmax(220px,1.5fr)_140px_auto] gap-3 lg:max-w-3xl"
          onSubmit={(event) => {
            event.preventDefault();
            onCreatePool();
          }}
        >
          <input
            value={poolDraft.name}
            onChange={(event) => onDraftChange({ ...poolDraft, name: event.target.value })}
            className="input"
            placeholder="Pool name"
          />
          <input
            value={poolDraft.description}
            onChange={(event) => onDraftChange({ ...poolDraft, description: event.target.value })}
            className="input"
            placeholder="Description"
          />
          <select
            value={poolDraft.strategy}
            onChange={(event) => onDraftChange({ ...poolDraft, strategy: event.target.value })}
            className="input"
          >
            <option value="weighted">Weighted</option>
            <option value="round_robin">Round robin</option>
          </select>
          <button type="submit" className="btn btn-primary whitespace-nowrap" disabled={!canCreate}>
            <PlusIcon className="w-5 h-5 mr-2" />
            {creatingPool ? 'Creating' : 'Create'}
          </button>
        </form>
      </div>

      <div className="mt-5 divide-y divide-apple-gray-100">
        {proxyPools.length === 0 ? (
          <div className="py-5 text-sm text-apple-gray-500">
            No proxy pools yet.
          </div>
        ) : (
          proxyPools.map((pool) => (
            <div key={pool.id} className="py-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-medium text-apple-gray-900 truncate">{pool.name}</span>
                  <span className="inline-flex items-center px-2 py-0.5 rounded-full bg-apple-gray-100 text-xs text-apple-gray-600">
                    {pool.active_proxy_count}/{pool.proxy_count} active
                  </span>
                  <span className="inline-flex items-center px-2 py-0.5 rounded-full bg-apple-gray-100 text-xs text-apple-gray-600">
                    {pool.strategy}
                  </span>
                </div>
                {pool.description && (
                  <p className="mt-1 text-sm text-apple-gray-500 truncate">{pool.description}</p>
                )}
              </div>
              <div className="flex items-center gap-3">
                <button
                  type="button"
                  onClick={() => onTogglePool(pool)}
                  className={`inline-flex items-center gap-1 px-2.5 py-1.5 rounded-full text-xs font-medium transition-colors ${
                    pool.is_active
                      ? 'bg-green-100 text-apple-green hover:bg-green-200'
                      : 'bg-gray-100 text-apple-gray-500 hover:bg-gray-200'
                  }`}
                  disabled={updatingPoolId === pool.id}
                >
                  {pool.is_active ? (
                    <CheckCircleIcon className="w-4 h-4" />
                  ) : (
                    <XCircleIcon className="w-4 h-4" />
                  )}
                  {updatingPoolId === pool.id ? 'Saving' : pool.is_active ? 'Active' : 'Inactive'}
                </button>
                <button
                  type="button"
                  onClick={() => onDeletePool(pool.id)}
                  className="text-apple-red hover:text-red-600 transition-colors inline-flex items-center gap-1 text-sm"
                  disabled={deletingPoolId === pool.id}
                >
                  <TrashIcon className="w-4 h-4" />
                  {deletingPoolId === pool.id ? 'Deleting' : 'Delete'}
                </button>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
}

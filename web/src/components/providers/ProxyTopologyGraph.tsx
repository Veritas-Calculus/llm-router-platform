import {
  ArrowRightIcon,
  CheckCircleIcon,
  ExclamationTriangleIcon,
  GlobeAltIcon,
  KeyIcon,
  ServerIcon,
  ServerStackIcon,
} from '@heroicons/react/24/outline';
import {
  ProviderAccountTopologyNode,
  Proxy,
  ProxyTopology,
  ProxyTopologyRouteStep,
} from '@/lib/types';

interface ProxyTopologyGraphProps {
  topology: ProxyTopology | null;
  selectedProviderId?: string | null;
}

const sourceLabels: Record<string, string> = {
  direct: 'Direct',
  api_key_proxy: 'Account proxy',
  api_key_proxy_pool: 'Account pool',
  provider_default_proxy: 'Provider proxy',
  provider_proxy_pool: 'Provider proxy pool',
};

export default function ProxyTopologyGraph({ topology, selectedProviderId }: ProxyTopologyGraphProps) {
  const providerNodes = (topology?.providers || []).filter((node) => (
    selectedProviderId ? node.provider.id === selectedProviderId : true
  ));
  const accountCount = providerNodes.reduce((sum, node) => sum + node.accounts.length, 0);
  const proxiedCount = providerNodes.reduce((sum, node) => (
    sum + node.accounts.filter((account) => account.bindingSource !== 'direct').length
  ), 0);

  return (
    <div className="card">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between mb-5">
        <div>
          <div className="flex items-center gap-2">
            <ServerStackIcon className="w-5 h-5 text-apple-blue" />
            <h3 className="text-lg font-semibold text-apple-gray-900">Proxy Topology</h3>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <TopologyMetric label="Accounts" value={accountCount} />
          <TopologyMetric label="Proxied" value={proxiedCount} />
          <TopologyMetric label="Direct" value={Math.max(accountCount - proxiedCount, 0)} />
        </div>
      </div>

      {providerNodes.length === 0 ? (
        <div className="py-8 text-center text-sm text-apple-gray-500">
          No topology data.
        </div>
      ) : (
        <div className="space-y-5">
          {providerNodes.map((node) => (
            <section key={node.provider.id} className="border border-apple-gray-100 rounded-xl">
              <div className="px-4 py-3 border-b border-apple-gray-100 flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <GlobeAltIcon className="w-4 h-4 text-apple-gray-500" />
                    <span className="font-medium text-apple-gray-900 truncate">{node.provider.name}</span>
                    <StatusBadge status={node.provider.is_active ? 'active' : 'inactive'} />
                  </div>
                  <p className="mt-1 text-xs text-apple-gray-500 truncate">{node.provider.base_url}</p>
                </div>
                <div className="flex flex-wrap gap-1.5">
                  {node.models.slice(0, 5).map((model) => (
                    <span key={model} className="px-2 py-0.5 rounded-full bg-apple-gray-100 text-xs text-apple-gray-600">
                      {model}
                    </span>
                  ))}
                  {node.models.length > 5 && (
                    <span className="px-2 py-0.5 rounded-full bg-apple-gray-100 text-xs text-apple-gray-600">
                      +{node.models.length - 5}
                    </span>
                  )}
                </div>
              </div>

              <div className="divide-y divide-apple-gray-100">
                {node.accounts.length === 0 ? (
                  <div className="px-4 py-5 text-sm text-apple-gray-500">No accounts.</div>
                ) : (
                  node.accounts.map((account, index) => (
                    <AccountRoute
                      key={account.apiKey?.id || `${node.provider.id}-${index}`}
                      account={account}
                    />
                  ))
                )}
              </div>
            </section>
          ))}
        </div>
      )}
    </div>
  );
}

function TopologyMetric({ label, value }: { label: string; value: number }) {
  return (
    <span className="inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-full bg-apple-gray-100 text-xs text-apple-gray-600">
      <span className="font-semibold text-apple-gray-900">{value}</span>
      {label}
    </span>
  );
}

function AccountRoute({ account }: { account: ProviderAccountTopologyNode }) {
  return (
    <div className="px-4 py-4">
      <div className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <KeyIcon className="w-4 h-4 text-apple-gray-500" />
            <span className="font-medium text-apple-gray-900 truncate">{account.label}</span>
            <span className="px-2 py-0.5 rounded-full bg-blue-50 text-apple-blue text-xs font-medium">
              {sourceLabels[account.bindingSource] || account.bindingSource}
            </span>
          </div>
          {account.apiKey?.key_prefix && (
            <code className="mt-1 inline-block text-xs bg-apple-gray-100 text-apple-gray-600 px-1.5 py-0.5 rounded">
              {account.apiKey.key_prefix}
            </code>
          )}
        </div>

        <div className="overflow-x-auto">
          <div className="flex items-center gap-2 min-w-max">
            {account.route.map((step, index) => (
              <RouteStepItem key={`${step.type}-${step.id}-${index}`} step={step} isLast={index === account.route.length - 1} />
            ))}
          </div>
        </div>
      </div>

      {account.candidateProxies.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-2">
          {account.candidateProxies.slice(0, 6).map((proxy) => (
            <ProxyCandidate key={proxy.id} proxy={proxy} />
          ))}
          {account.candidateProxies.length > 6 && (
            <span className="px-2 py-1 rounded-lg bg-apple-gray-100 text-xs text-apple-gray-500">
              +{account.candidateProxies.length - 6} proxies
            </span>
          )}
        </div>
      )}
    </div>
  );
}

function RouteStepItem({ step, isLast }: { step: ProxyTopologyRouteStep; isLast: boolean }) {
  return (
    <>
      <div className="w-44 rounded-xl border border-apple-gray-200 bg-[var(--theme-bg-surface)] px-3 py-2">
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-2 min-w-0">
            <StepIcon type={step.type} status={step.status} />
            <span className="text-xs font-medium text-apple-gray-900 truncate">{step.label}</span>
          </div>
          <StatusDot status={step.status} />
        </div>
        {step.detail && (
          <p className="mt-1 text-[11px] text-apple-gray-500 truncate">{step.detail}</p>
        )}
      </div>
      {!isLast && <ArrowRightIcon className="w-4 h-4 shrink-0 text-apple-gray-300" />}
    </>
  );
}

function StepIcon({ type, status }: { type: string; status: string }) {
  const className = `w-4 h-4 shrink-0 ${status === 'active' ? 'text-apple-blue' : 'text-apple-gray-400'}`;
  if (type === 'account') return <KeyIcon className={className} />;
  if (type === 'proxy' || type === 'proxy_pool') return <ServerIcon className={className} />;
  return <GlobeAltIcon className={className} />;
}

function StatusBadge({ status }: { status: string }) {
  const label = status === 'active' ? 'Active' : status === 'inactive' ? 'Inactive' : status;
  return (
    <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${statusBadgeClass(status)}`}>
      {status === 'active' ? <CheckCircleIcon className="w-3.5 h-3.5" /> : <ExclamationTriangleIcon className="w-3.5 h-3.5" />}
      {label}
    </span>
  );
}

function StatusDot({ status }: { status: string }) {
  const color = status === 'active'
    ? 'bg-apple-green'
    : status === 'missing' || status === 'cycle'
    ? 'bg-apple-red'
    : 'bg-apple-orange';
  return <span className={`w-2 h-2 rounded-full shrink-0 ${color}`} title={status} />;
}

function statusBadgeClass(status: string): string {
  if (status === 'active') return 'bg-green-100 text-apple-green';
  if (status === 'missing' || status === 'cycle') return 'bg-red-100 text-apple-red';
  return 'bg-orange-100 text-apple-orange';
}

function ProxyCandidate({ proxy }: { proxy: Proxy }) {
  return (
    <span className="inline-flex items-center gap-1 px-2 py-1 rounded-lg bg-apple-gray-100 text-xs text-apple-gray-600 max-w-[220px]">
      <ServerIcon className="w-3.5 h-3.5 shrink-0 text-apple-gray-400" />
      <span className="truncate">{proxy.url}</span>
      {proxy.region && <span className="text-apple-gray-400">({proxy.region})</span>}
    </span>
  );
}

import { motion } from 'framer-motion';
import { CheckCircleIcon, XCircleIcon } from '@heroicons/react/24/outline';
import { Provider } from '@/lib/api';

interface ProviderListProps {
  providers: Provider[];
  selectedProvider: Provider | null;
  onSelect: (provider: Provider) => void;
  onToggle: (provider: Provider) => void;
}

export default function ProviderList({
  providers,
  selectedProvider,
  onSelect,
  onToggle,
}: ProviderListProps) {
  return (
    <motion.div
      initial={{ opacity: 0, x: -10 }}
      animate={{ opacity: 1, x: 0 }}
      className="lg:col-span-1"
    >
      <div className="card">
        <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">Providers</h2>
        <div className="space-y-2">
          {providers.map((provider) => (
            <div
              key={provider.id}
              className={`flex items-center justify-between px-4 py-3 rounded-apple transition-colors cursor-pointer ${selectedProvider?.id === provider.id
                ? 'bg-apple-blue text-white'
                : 'hover:bg-apple-gray-100 text-apple-gray-900'
                }`}
              onClick={() => onSelect(provider)}
            >
              <div className="flex-1 min-w-0">
                <p className="font-medium truncate">{provider.name}</p>
                <p
                  className={`text-sm ${selectedProvider?.id === provider.id
                    ? 'text-white/80'
                    : 'text-apple-gray-500'
                    }`}
                >
                  Priority: {provider.priority}
                </p>
              </div>
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  onToggle(provider);
                }}
                className={`ml-2 p-1 rounded-full transition-colors ${selectedProvider?.id === provider.id
                  ? 'hover:bg-white/20'
                  : 'hover:bg-apple-gray-200'
                  }`}
                title={provider.is_active ? 'Disable provider' : 'Enable provider'}
              >
                {provider.is_active ? (
                  <CheckCircleIcon
                    className={`w-5 h-5 ${selectedProvider?.id === provider.id ? 'text-white' : 'text-apple-green'
                      }`}
                  />
                ) : (
                  <XCircleIcon
                    className={`w-5 h-5 ${selectedProvider?.id === provider.id ? 'text-white/60' : 'text-apple-gray-400'
                      }`}
                  />
                )}
              </button>
            </div>
          ))}
        </div>
      </div>
    </motion.div>
  );
}

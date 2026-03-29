/* eslint-disable @typescript-eslint/no-explicit-any */
import {
  ShieldCheckIcon, SparklesIcon, LockClosedIcon, AdjustmentsHorizontalIcon,
  EnvelopeIcon, DevicePhoneMobileIcon, CreditCardIcon, IdentificationIcon, KeyIcon,
} from '@heroicons/react/24/outline';

/* ── PolicyPreset Type ── */

export interface PolicyPreset {
  id: string;
  name: string;
  description: string;
  icon: React.ReactNode;
  color: string;
  borderColor: string;
  bgColor: string;
  config: {
    isEnabled: boolean;
    strategy: 'REDACT' | 'BLOCK';
    maskEmails: boolean;
    maskPhones: boolean;
    maskCreditCards: boolean;
    maskSsn: boolean;
    maskApiKeys: boolean;
  };
}

/* ── Presets ── */

export const POLICY_PRESETS: PolicyPreset[] = [
  {
    id: 'relaxed', name: 'Relaxed',
    description: 'Minimal protection — only masks credit cards and SSNs. Suitable for internal development environments.',
    icon: <SparklesIcon className="w-5 h-5" />, color: 'text-blue-500', borderColor: 'border-blue-500', bgColor: 'bg-blue-500/10',
    config: { isEnabled: true, strategy: 'REDACT', maskEmails: false, maskPhones: false, maskCreditCards: true, maskSsn: true, maskApiKeys: false },
  },
  {
    id: 'standard', name: 'Standard',
    description: 'Balanced protection — redacts emails, phones, cards, and SSNs. Recommended for most production workloads.',
    icon: <ShieldCheckIcon className="w-5 h-5" />, color: 'text-green-500', borderColor: 'border-green-500', bgColor: 'bg-green-500/10',
    config: { isEnabled: true, strategy: 'REDACT', maskEmails: true, maskPhones: true, maskCreditCards: true, maskSsn: true, maskApiKeys: false },
  },
  {
    id: 'strict', name: 'Strict',
    description: 'Maximum protection — blocks any request containing PII. Use for regulated industries (HIPAA, PCI-DSS).',
    icon: <LockClosedIcon className="w-5 h-5" />, color: 'text-red-500', borderColor: 'border-red-500', bgColor: 'bg-red-500/10',
    config: { isEnabled: true, strategy: 'BLOCK', maskEmails: true, maskPhones: true, maskCreditCards: true, maskSsn: true, maskApiKeys: true },
  },
  {
    id: 'custom', name: 'Custom',
    description: 'Fine-tune each rule individually. Full control over strategy, patterns, and custom regex.',
    icon: <AdjustmentsHorizontalIcon className="w-5 h-5" />, color: 'text-purple-500', borderColor: 'border-purple-500', bgColor: 'bg-purple-500/10',
    config: { isEnabled: true, strategy: 'REDACT', maskEmails: true, maskPhones: true, maskCreditCards: true, maskSsn: true, maskApiKeys: true },
  },
];

/* ── Detect active preset ── */

export function detectActivePreset(config: any): string {
  if (!config?.isEnabled) return 'none';
  for (const preset of POLICY_PRESETS) {
    if (preset.id === 'custom') continue;
    const p = preset.config;
    if (config.strategy === p.strategy && config.maskEmails === p.maskEmails && config.maskPhones === p.maskPhones && config.maskCreditCards === p.maskCreditCards && config.maskSsn === p.maskSsn && config.maskApiKeys === p.maskApiKeys) {
      return preset.id;
    }
  }
  return 'custom';
}

/* ── PII Rules ── */

export const PII_RULES = [
  { field: 'maskEmails', label: 'Email Addresses', desc: 'user@example.com, admin@corp.org', icon: EnvelopeIcon },
  { field: 'maskPhones', label: 'Phone Numbers', desc: '+1 (555) 123-4567, 138-0000-0000', icon: DevicePhoneMobileIcon },
  { field: 'maskCreditCards', label: 'Credit Card Numbers', desc: '4111-1111-1111-1111 (16-digit PANs)', icon: CreditCardIcon },
  { field: 'maskSsn', label: 'Social Security Numbers', desc: '123-45-6789 (US SSN format)', icon: IdentificationIcon },
  { field: 'maskApiKeys', label: 'API Keys & Secrets', desc: 'sk-..., Bearer tokens, AWS keys', icon: KeyIcon },
];

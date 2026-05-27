import { useEffect, useMemo } from 'react';
import { useQuery } from '@apollo/client/react';
import { useAuthStore } from '@/stores/authStore';
import { useAuthHydrated } from '@/hooks/useAuthHydrated';
import { ChevronUpDownIcon } from '@heroicons/react/24/outline';
import { MY_ORGANIZATIONS } from '@/lib/graphql/operations';

export default function OrgSwitcher() {
  const { selectedOrgId, setSelectedOrgId } = useAuthStore();
  const { data } = useQuery(MY_ORGANIZATIONS);
  const orgs = useMemo(() => data?.myOrganizations || [], [data]);
  const hydrated = useAuthHydrated();

  // Auto-select first org — gated on hydration so we never overwrite a
  // persisted selectedOrgId that the rehydration is about to restore.
  useEffect(() => {
    if (!hydrated) return;
    if (orgs.length > 0 && !selectedOrgId) {
      setSelectedOrgId(orgs[0].id);
    }
  }, [hydrated, orgs, selectedOrgId, setSelectedOrgId]);

  if (orgs.length < 2) return null;

  return (
    <div className="px-4 pb-2">
      <label htmlFor="org-switcher" className="sr-only">
        Select active organization
      </label>
      <div className="relative">
        <select
          id="org-switcher"
          value={selectedOrgId || ''}
          onChange={(e) => setSelectedOrgId(e.target.value)}
          className="w-full appearance-none px-3 py-2 pr-8 bg-apple-gray-50 border border-apple-gray-200 rounded-xl text-sm font-medium text-apple-gray-700 focus:ring-2 focus:ring-apple-blue focus:border-transparent cursor-pointer"
        >
          {orgs.map((org) => (
            <option key={org.id} value={org.id}>{org.name}</option>
          ))}
        </select>
        <ChevronUpDownIcon aria-hidden="true" className="absolute right-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-apple-gray-400 pointer-events-none" />
      </div>
    </div>
  );
}

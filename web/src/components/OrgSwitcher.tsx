import { useEffect, useMemo } from 'react';
import { useQuery } from '@apollo/client/react';
import { gql } from '@apollo/client';
import { useAuthStore } from '@/stores/authStore';
import { ChevronUpDownIcon } from '@heroicons/react/24/outline';

/* eslint-disable @typescript-eslint/no-explicit-any */

const MY_ORGS = gql`query MyOrgsForSwitcher { myOrganizations { id name } }`;

export default function OrgSwitcher() {
  const { selectedOrgId, setSelectedOrgId } = useAuthStore();
  const { data } = useQuery<any>(MY_ORGS);
  const orgs = useMemo(() => data?.myOrganizations || [], [data]);

  // Auto-select first org
  useEffect(() => {
    if (orgs.length > 0 && !selectedOrgId) {
      setSelectedOrgId(orgs[0].id);
    }
  }, [orgs, selectedOrgId, setSelectedOrgId]);

  if (orgs.length < 2) return null;

  return (
    <div className="px-4 pb-2">
      <div className="relative">
        <select
          value={selectedOrgId || ''}
          onChange={(e) => setSelectedOrgId(e.target.value)}
          className="w-full appearance-none px-3 py-2 pr-8 bg-apple-gray-50 border border-apple-gray-200 rounded-xl text-sm font-medium text-apple-gray-700 focus:ring-2 focus:ring-apple-blue focus:border-transparent cursor-pointer"
        >
          {orgs.map((org: any) => (
            <option key={org.id} value={org.id}>{org.name}</option>
          ))}
        </select>
        <ChevronUpDownIcon className="absolute right-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-apple-gray-400 pointer-events-none" />
      </div>
    </div>
  );
}

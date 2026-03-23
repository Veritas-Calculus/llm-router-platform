import { useState, useMemo } from 'react';
import { motion } from 'framer-motion';
import toast from 'react-hot-toast';
import {
    PlusIcon,
    TrashIcon,
    ShieldCheckIcon,
    UserIcon,
} from '@heroicons/react/24/outline';
import { useQuery, useMutation } from '@apollo/client/react';
import {
    MY_API_KEYS, // using this to just load myOrganizations
    GET_ORG_MEMBERS,
    ADD_ORG_MEMBER,
    UPDATE_ORG_MEMBER_ROLE,
    REMOVE_ORG_MEMBER,
} from '@/lib/graphql/operations';
import { useAuthStore } from '@/stores/authStore';
import { useTranslation } from '@/lib/i18n';

/* eslint-disable @typescript-eslint/no-explicit-any */

const ROLES = [
    { value: 'OWNER', label: 'Owner', desc: 'Full access including billing and deletion' },
    { value: 'ADMIN', label: 'Admin', desc: 'Can manage keys, members, settings' },
    { value: 'MEMBER', label: 'Member', desc: 'Can create and manage own keys' },
    { value: 'READONLY', label: 'Read Only', desc: 'Can view resources but not modify' },
];

export default function OrganizationMembersPage() {
  const { t } = useTranslation();
    const { user } = useAuthStore();
    
    // We get the orgs from MY_API_KEYS query context currently 
    // Ideally we should have a standalone GET_MY_ORGS query, but this works for now.
    const { data: orgData, loading: orgLoading } = useQuery<any>(MY_API_KEYS);
    
    const orgs = useMemo(() => orgData?.myOrganizations || [], [orgData]);
    const [selectedOrgId, setSelectedOrgId] = useState<string>('');

    // Pre-select first org
    useMemo(() => {
        if (orgs.length > 0 && !selectedOrgId) {
            setSelectedOrgId(orgs[0].id);
        }
    }, [orgs, selectedOrgId]);

    const { data: membersData, loading: membersLoading, refetch } = useQuery<any>(GET_ORG_MEMBERS, {
        variables: { orgId: selectedOrgId },
        skip: !selectedOrgId,
    });

    const [addMember] = useMutation(ADD_ORG_MEMBER);
    const [updateRole] = useMutation(UPDATE_ORG_MEMBER_ROLE);
    const [removeMember] = useMutation(REMOVE_ORG_MEMBER);

    const [isAddModalOpen, setIsAddModalOpen] = useState(false);
    const [newMemberEmail, setNewMemberEmail] = useState('');
    const [newMemberRole, setNewMemberRole] = useState('MEMBER');

    const members = membersData?.organizationMembers || [];

    const handleAddMember = async (e: React.FormEvent) => {
        e.preventDefault();
        try {
            await addMember({
                variables: { orgId: selectedOrgId, email: newMemberEmail, role: newMemberRole },
            });
            toast.success(t('org_members.member_added'));
            setIsAddModalOpen(false);
            setNewMemberEmail('');
            refetch();
        } catch (err: any) {
            toast.error(err.message || t('org_members.add_error'));
        }
    };

    const handleUpdateRole = async (userId: string, targetRole: string) => {
        try {
            await updateRole({
                variables: { orgId: selectedOrgId, userId, role: targetRole },
            });
            toast.success(t('org_members.role_updated'));
            refetch();
        } catch (err: any) {
            toast.error(err.message || t('org_members.role_update_error'));
        }
    };

    const handleRemoveMember = async (userId: string) => {
        if (!confirm("Are you sure you want to remove this member?")) return;
        try {
            await removeMember({
                variables: { orgId: selectedOrgId, userId },
            });
            toast.success(t('org_members.member_removed'));
            refetch();
        } catch (err: any) {
            toast.error(err.message || t('org_members.remove_error'));
        }
    };

    if (orgLoading) return <div className="p-8 text-center text-apple-gray-500">Loading organization context...</div>;

    return (
        <div className="max-w-6xl mx-auto pb-12">
            <div className="flex items-center justify-between mb-8">
                <div>
                    <h1 className="text-2xl font-semibold text-apple-gray-900">Organization Members</h1>
                    <p className="text-apple-gray-500 mt-1">Manage users, access, and RBAC roles in your organization.</p>
                </div>
                <button
                    onClick={() => setIsAddModalOpen(true)}
                    className="btn btn-primary"
                    disabled={!selectedOrgId}
                >
                    <PlusIcon className="w-5 h-5 mr-2" />
                    Invite Member
                </button>
            </div>

            <div className="mb-6 flex items-center gap-4">
                <label className="text-sm font-medium text-apple-gray-700">Select Organization:</label>
                <select
                    value={selectedOrgId}
                    onChange={(e) => setSelectedOrgId(e.target.value)}
                    className="input max-w-xs"
                >
                    {orgs.map((org: any) => (
                        <option key={org.id} value={org.id}>{org.name}</option>
                    ))}
                </select>
            </div>

            <div className="card overflow-x-auto">
                <table className="w-full text-left">
                    <thead>
                        <tr className="border-b border-apple-gray-200">
                            <th className="py-3 px-4 text-sm font-medium text-apple-gray-500">Current Members</th>
                            <th className="py-3 px-4 text-sm font-medium text-apple-gray-500">Role</th>
                            <th className="py-3 px-4 text-sm font-medium text-apple-gray-500">Joined</th>
                            <th className="py-3 px-4 text-sm font-medium text-apple-gray-500 text-right">Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        {membersLoading ? (
                            <tr><td colSpan={4} className="py-8 text-center text-apple-gray-500">Loading members...</td></tr>
                        ) : members.length === 0 ? (
                            <tr><td colSpan={4} className="py-8 text-center text-apple-gray-500">No members found.</td></tr>
                        ) : (
                            members.map((m: any, idx: number) => (
                                <motion.tr 
                                    key={m.userId}
                                    initial={{ opacity: 0, y: 10 }}
                                    animate={{ opacity: 1, y: 0 }}
                                    transition={{ delay: idx * 0.05 }}
                                    className="border-b border-apple-gray-100 hover:bg-apple-gray-50 transition-colors"
                                >
                                    <td className="py-4 px-4">
                                        <div className="flex items-center gap-3">
                                            <div className="w-10 h-10 rounded-full bg-apple-blue/10 flex items-center justify-center text-apple-blue font-medium">
                                                {m.user.name.charAt(0).toUpperCase()}
                                            </div>
                                            <div>
                                                <div className="font-medium text-apple-gray-900">
                                                    {m.user.name} {m.userId === user?.id && <span className="text-xs ml-2 text-apple-blue bg-apple-blue/10 px-2 py-0.5 rounded-full">You</span>}
                                                </div>
                                                <div className="text-sm text-apple-gray-500">{m.user.email}</div>
                                            </div>
                                        </div>
                                    </td>
                                    <td className="py-4 px-4">
                                        <select
                                            value={m.role}
                                            onChange={(e) => handleUpdateRole(m.userId, e.target.value)}
                                            disabled={m.userId === user?.id} // Can't edit own role
                                            className="input text-sm py-1.5 px-3"
                                        >
                                            {ROLES.map(r => (
                                                <option key={r.value} value={r.value}>{r.label}</option>
                                            ))}
                                        </select>
                                    </td>
                                    <td className="py-4 px-4 text-sm text-apple-gray-500">
                                        {new Date(m.createdAt).toLocaleDateString()}
                                    </td>
                                    <td className="py-4 px-4 text-right">
                                        <button
                                            onClick={() => handleRemoveMember(m.userId)}
                                            disabled={m.userId === user?.id}
                                            className="p-2 text-apple-gray-400 hover:text-red-600 disabled:opacity-30 disabled:hover:text-apple-gray-400 transition-colors rounded-lg hover:bg-apple-gray-100"
                                            title={t('org_members.remove_member')}
                                        >
                                            <TrashIcon className="w-5 h-5" />
                                        </button>
                                    </td>
                                </motion.tr>
                            ))
                        )}
                    </tbody>
                </table>
            </div>

            {/* Add Member Modal */}
            {isAddModalOpen && (
                <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm p-4">
                    <motion.div 
                        initial={{ opacity: 0, scale: 0.95 }}
                        animate={{ opacity: 1, scale: 1 }}
                        className="bg-white rounded-2xl shadow-apple-lg w-full max-w-md overflow-hidden"
                    >
                        <div className="px-6 py-4 border-b border-apple-gray-100 flex items-center justify-between">
                            <h3 className="text-lg font-semibold text-apple-gray-900">Add Team Member</h3>
                        </div>
                        <form onSubmit={handleAddMember} className="p-6">
                            <div className="space-y-4">
                                <div>
                                    <label className="block text-sm font-medium text-apple-gray-700 mb-1">
                                        User Email Address
                                    </label>
                                    <input
                                        type="email"
                                        required
                                        value={newMemberEmail}
                                        onChange={(e) => setNewMemberEmail(e.target.value)}
                                        className="input w-full"
                                        placeholder="colleague@company.com"
                                    />
                                    <p className="text-xs text-apple-gray-500 mt-1">
                                        The user must already be registered in the system.
                                    </p>
                                </div>
                                <div>
                                    <label className="block text-sm font-medium text-apple-gray-700 mb-1">
                                        Role
                                    </label>
                                    <div className="grid gap-3">
                                        {ROLES.map(role => (
                                            <label 
                                                key={role.value}
                                                className={`border rounded-xl p-3 cursor-pointer flex gap-3 transition-colors ${
                                                    newMemberRole === role.value ? 'border-apple-blue bg-apple-blue/5' : 'border-apple-gray-200 hover:border-apple-gray-300'
                                                }`}
                                            >
                                                <input 
                                                    type="radio" 
                                                    name="role" 
                                                    value={role.value} 
                                                    checked={newMemberRole === role.value}
                                                    onChange={() => setNewMemberRole(role.value)}
                                                    className="mt-1"
                                                />
                                                <div>
                                                    <div className="font-medium text-sm text-apple-gray-900 flex items-center gap-1">
                                                        {role.value === 'ADMIN' ? <ShieldCheckIcon className="w-4 h-4 text-apple-blue" /> : <UserIcon className="w-4 h-4 text-apple-gray-500" />}
                                                        {role.label}
                                                    </div>
                                                    <div className="text-xs text-apple-gray-500 mt-0.5">{role.desc}</div>
                                                </div>
                                            </label>
                                        ))}
                                    </div>
                                </div>
                            </div>
                            <div className="mt-8 flex justify-end gap-3">
                                <button
                                    type="button"
                                    onClick={() => setIsAddModalOpen(false)}
                                    className="btn btn-secondary"
                                >
                                    Cancel
                                </button>
                                <button type="submit" className="btn btn-primary">
                                    Add Member
                                </button>
                            </div>
                        </form>
                    </motion.div>
                </div>
            )}
        </div>
    );
}

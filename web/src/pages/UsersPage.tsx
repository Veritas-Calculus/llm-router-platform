import { useState, useMemo, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { motion } from 'framer-motion';
import toast from 'react-hot-toast';
import {
    MagnifyingGlassIcon,
    ShieldCheckIcon,
    NoSymbolIcon,
    CheckCircleIcon,
} from '@heroicons/react/24/outline';
import { useQuery, useMutation } from '@apollo/client/react';
import { USERS_QUERY, TOGGLE_USER, UPDATE_USER_ROLE } from '@/lib/graphql/operations';
import type { UserListItem } from '@/lib/types';

/* eslint-disable @typescript-eslint/no-explicit-any */

function UsersPage() {
    const navigate = useNavigate();
    const { data, loading, refetch } = useQuery<any>(USERS_QUERY);
    const [toggleUserMut] = useMutation(TOGGLE_USER);
    const [updateRoleMut] = useMutation(UPDATE_USER_ROLE);
    const [searchQuery, setSearchQuery] = useState('');

    const users: UserListItem[] = useMemo(() =>
        (data?.users?.data || []).map((u: any) => ({
            id: u.id, name: u.name, email: u.email, role: u.role,
            is_active: u.isActive, api_key_count: u.apiKeyCount, created_at: u.createdAt,
        })),
    [data]);
    const total = data?.users?.total || 0;

    const handleSearch = (e: React.FormEvent) => {
        e.preventDefault();
        refetch({ search: searchQuery || undefined });
    };

    const handleToggle = useCallback(async (id: string, name: string) => {
        try {
            const { data: result } = await toggleUserMut({ variables: { id } });
            toast.success(`${name} ${(result as any)?.toggleUser?.isActive ? 'enabled' : 'disabled'}`);
            refetch();
        } catch {
            toast.error('Failed to toggle user');
        }
    }, [toggleUserMut, refetch]);

    const handleRoleChange = useCallback(async (id: string, name: string, currentRole: string) => {
        const newRole = currentRole === 'admin' ? 'user' : 'admin';
        try {
            await updateRoleMut({ variables: { id, role: newRole } });
            toast.success(`${name} role changed to ${newRole}`);
            refetch();
        } catch {
            toast.error('Failed to update role');
        }
    }, [updateRoleMut, refetch]);

    const formatDate = (dateStr: string) => {
        if (!dateStr || dateStr === '0001-01-01T00:00:00Z') return 'Never';
        return new Date(dateStr).toLocaleDateString('en-US', {
            month: 'short',
            day: 'numeric',
            year: 'numeric',
        });
    };

    return (
        <div>
            <div className="flex items-center justify-between mb-8">
                <div>
                    <h1 className="text-2xl font-semibold text-apple-gray-900">User Management</h1>
                    <p className="text-apple-gray-500 mt-1">{total} registered users</p>
                </div>

                <form onSubmit={handleSearch} className="flex gap-2">
                    <div className="relative">
                        <MagnifyingGlassIcon className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-apple-gray-400" />
                        <input
                            type="text"
                            value={searchQuery}
                            onChange={(e) => setSearchQuery(e.target.value)}
                            placeholder="Search by name or email..."
                            className="input pl-9 w-72"
                        />
                    </div>
                    <button type="submit" className="btn-primary px-4">
                        Search
                    </button>
                </form>
            </div>

            <div className="card overflow-hidden">
                <table className="w-full">
                    <thead>
                        <tr className="border-b border-apple-gray-200">
                            <th className="text-left py-3 px-4 text-sm font-medium text-apple-gray-500">User</th>
                            <th className="text-left py-3 px-4 text-sm font-medium text-apple-gray-500">Role</th>
                            <th className="text-left py-3 px-4 text-sm font-medium text-apple-gray-500">Status</th>
                            <th className="text-left py-3 px-4 text-sm font-medium text-apple-gray-500">API Keys</th>
                            <th className="text-left py-3 px-4 text-sm font-medium text-apple-gray-500">Registered</th>
                            <th className="text-right py-3 px-4 text-sm font-medium text-apple-gray-500">Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        {loading ? (
                            <tr>
                                <td colSpan={6} className="py-12 text-center text-apple-gray-400">
                                    Loading users...
                                </td>
                            </tr>
                        ) : users.length === 0 ? (
                            <tr>
                                <td colSpan={6} className="py-12 text-center text-apple-gray-400">
                                    No users found
                                </td>
                            </tr>
                        ) : (
                            users.map((user, idx) => (
                                <motion.tr
                                    key={user.id}
                                    initial={{ opacity: 0, y: 10 }}
                                    animate={{ opacity: 1, y: 0 }}
                                    transition={{ delay: idx * 0.03 }}
                                    className="border-b border-apple-gray-100 hover:bg-apple-gray-50 cursor-pointer transition-colors"
                                    onClick={() => navigate(`/users/${user.id}`)}
                                >
                                    <td className="py-3 px-4">
                                        <div className="flex items-center gap-3">
                                            <div className={`w-9 h-9 rounded-full flex items-center justify-center ${user.role === 'admin' ? 'bg-amber-500' : 'bg-apple-blue'}`}>
                                                <span className="text-white text-sm font-medium">
                                                    {user.name?.charAt(0).toUpperCase() || '?'}
                                                </span>
                                            </div>
                                            <div>
                                                <p className="text-sm font-medium text-apple-gray-900">{user.name}</p>
                                                <p className="text-xs text-apple-gray-500">{user.email}</p>
                                            </div>
                                        </div>
                                    </td>
                                    <td className="py-3 px-4">
                                        <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${user.role === 'admin' ? 'bg-amber-100 text-amber-700' : 'bg-blue-100 text-blue-700'}`}>
                                            {user.role === 'admin' && <ShieldCheckIcon className="w-3 h-3" />}
                                            {user.role}
                                        </span>
                                    </td>
                                    <td className="py-3 px-4">
                                        <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${user.is_active ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
                                            {user.is_active ? (
                                                <><CheckCircleIcon className="w-3 h-3" /> Active</>
                                            ) : (
                                                <><NoSymbolIcon className="w-3 h-3" /> Disabled</>
                                            )}
                                        </span>
                                    </td>
                                    <td className="py-3 px-4 text-sm text-apple-gray-600">
                                        {user.api_key_count}
                                    </td>
                                    <td className="py-3 px-4 text-sm text-apple-gray-500">
                                        {formatDate(user.created_at)}
                                    </td>
                                    <td className="py-3 px-4 text-right">
                                        <div className="flex items-center justify-end gap-2" onClick={(e) => e.stopPropagation()}>
                                            <button
                                                onClick={() => handleToggle(user.id, user.name)}
                                                className={`text-xs px-2 py-1 rounded-md transition-colors ${user.is_active ? 'text-red-600 hover:bg-red-50' : 'text-green-600 hover:bg-green-50'}`}
                                            >
                                                {user.is_active ? 'Disable' : 'Enable'}
                                            </button>
                                            <button
                                                onClick={() => handleRoleChange(user.id, user.name, user.role)}
                                                className="text-xs px-2 py-1 rounded-md text-apple-gray-600 hover:bg-apple-gray-100 transition-colors"
                                            >
                                                {user.role === 'admin' ? 'Demote' : 'Promote'}
                                            </button>
                                            <button
                                                onClick={() => navigate(`/users/${user.id}`)}
                                                className="text-xs px-2 py-1 rounded-md text-apple-blue hover:bg-blue-50 transition-colors"
                                            >
                                                Details
                                            </button>
                                        </div>
                                    </td>
                                </motion.tr>
                            ))
                        )}
                    </tbody>
                </table>
            </div>
        </div>
    );
}

export default UsersPage;

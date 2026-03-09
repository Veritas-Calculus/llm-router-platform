import { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { motion } from 'framer-motion';
import toast from 'react-hot-toast';
import {
    ArrowLeftIcon,
    ShieldCheckIcon,
    KeyIcon,
    ChartBarIcon,
    CurrencyDollarIcon,
} from '@heroicons/react/24/outline';
import {
    BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
} from 'recharts';
import { usersApi, UserDetail, DailyStats, ApiKey } from '@/lib/api';

function UserDetailPage() {
    const { id } = useParams<{ id: string }>();
    const navigate = useNavigate();
    const [user, setUser] = useState<UserDetail | null>(null);
    const [usage, setUsage] = useState<DailyStats[]>([]);
    const [apiKeys, setApiKeys] = useState<ApiKey[]>([]);
    const [loading, setLoading] = useState(true);
    const [tokenLimit, setTokenLimit] = useState('');
    const [budgetLimit, setBudgetLimit] = useState('');

    const fetchData = useCallback(async () => {
        if (!id) return;
        setLoading(true);
        try {
            const [userData, usageData, keysData] = await Promise.all([
                usersApi.getById(id),
                usersApi.getUsage(id, 30),
                usersApi.getApiKeys(id),
            ]);
            setUser(userData);
            setUsage(usageData.data || []);
            setApiKeys(keysData.data || []);
            setTokenLimit(String(userData.monthly_token_limit || 0));
            setBudgetLimit(String(userData.monthly_budget_usd || 0));
        } catch {
            toast.error('Failed to load user details');
        } finally {
            setLoading(false);
        }
    }, [id]);

    useEffect(() => {
        fetchData();
    }, [fetchData]);

    const handleToggle = async () => {
        if (!id || !user) return;
        try {
            const res = await usersApi.toggle(id);
            toast.success(`${user.name} ${res.is_active ? 'enabled' : 'disabled'}`);
            fetchData();
        } catch {
            toast.error('Failed to toggle user');
        }
    };

    const handleRoleChange = async () => {
        if (!id || !user) return;
        const newRole = user.role === 'admin' ? 'user' : 'admin';
        try {
            await usersApi.updateRole(id, newRole);
            toast.success(`Role changed to ${newRole}`);
            fetchData();
        } catch {
            toast.error('Failed to change role');
        }
    };

    const handleSaveQuota = async () => {
        if (!id) return;
        try {
            await usersApi.updateQuota(id, {
                monthly_token_limit: parseInt(tokenLimit) || 0,
                monthly_budget_usd: parseFloat(budgetLimit) || 0,
            });
            toast.success('Quota updated');
            fetchData();
        } catch {
            toast.error('Failed to update quota');
        }
    };

    if (loading) {
        return (
            <div className="flex items-center justify-center h-64">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue"></div>
            </div>
        );
    }

    if (!user) {
        return (
            <div className="text-center py-16">
                <p className="text-apple-gray-500">User not found</p>
                <button onClick={() => navigate('/users')} className="btn-primary mt-4">
                    Back to Users
                </button>
            </div>
        );
    }

    const tokenUsagePct = user.monthly_token_limit > 0
        ? Math.min(100, ((user.usage_month?.total_tokens || 0) / user.monthly_token_limit) * 100)
        : 0;
    const budgetUsagePct = user.monthly_budget_usd > 0
        ? Math.min(100, ((user.usage_month?.total_cost || 0) / user.monthly_budget_usd) * 100)
        : 0;

    const stats = [
        {
            label: 'Requests (Month)',
            value: user.usage_month?.total_requests?.toLocaleString() || '0',
            icon: ChartBarIcon,
            color: 'text-blue-600 bg-blue-50',
        },
        {
            label: 'Tokens (Month)',
            value: user.usage_month?.total_tokens?.toLocaleString() || '0',
            icon: ChartBarIcon,
            color: 'text-purple-600 bg-purple-50',
        },
        {
            label: 'Cost (Month)',
            value: `$${(user.usage_month?.total_cost || 0).toFixed(4)}`,
            icon: CurrencyDollarIcon,
            color: 'text-green-600 bg-green-50',
        },
        {
            label: 'API Keys',
            value: user.api_keys?.toString() || '0',
            icon: KeyIcon,
            color: 'text-amber-600 bg-amber-50',
        },
    ];

    return (
        <div>
            {/* Header */}
            <div className="flex items-center gap-4 mb-8">
                <button
                    onClick={() => navigate('/users')}
                    className="p-2 rounded-apple hover:bg-apple-gray-100 transition-colors"
                >
                    <ArrowLeftIcon className="w-5 h-5 text-apple-gray-600" />
                </button>
                <div className="flex-1">
                    <div className="flex items-center gap-3">
                        <div className={`w-12 h-12 rounded-full flex items-center justify-center ${user.role === 'admin' ? 'bg-amber-500' : 'bg-apple-blue'}`}>
                            <span className="text-white text-lg font-semibold">
                                {user.name?.charAt(0).toUpperCase() || '?'}
                            </span>
                        </div>
                        <div>
                            <h1 className="text-2xl font-semibold text-apple-gray-900">{user.name}</h1>
                            <p className="text-apple-gray-500">{user.email}</p>
                        </div>
                        <span className={`ml-2 inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-medium ${user.role === 'admin' ? 'bg-amber-100 text-amber-700' : 'bg-blue-100 text-blue-700'}`}>
                            {user.role === 'admin' && <ShieldCheckIcon className="w-3.5 h-3.5" />}
                            {user.role}
                        </span>
                        <span className={`inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium ${user.is_active ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
                            {user.is_active ? 'Active' : 'Disabled'}
                        </span>
                    </div>
                </div>
                <div className="flex gap-2">
                    <button
                        onClick={handleToggle}
                        className={`px-4 py-2 rounded-apple text-sm font-medium transition-colors ${user.is_active ? 'bg-red-50 text-red-600 hover:bg-red-100' : 'bg-green-50 text-green-600 hover:bg-green-100'}`}
                    >
                        {user.is_active ? 'Disable Account' : 'Enable Account'}
                    </button>
                    <button
                        onClick={handleRoleChange}
                        className="px-4 py-2 rounded-apple text-sm font-medium bg-apple-gray-100 text-apple-gray-700 hover:bg-apple-gray-200 transition-colors"
                    >
                        {user.role === 'admin' ? 'Demote to User' : 'Promote to Admin'}
                    </button>
                </div>
            </div>

            {/* Stats Cards */}
            <div className="grid grid-cols-4 gap-4 mb-8">
                {stats.map((stat, idx) => (
                    <motion.div
                        key={stat.label}
                        initial={{ opacity: 0, y: 10 }}
                        animate={{ opacity: 1, y: 0 }}
                        transition={{ delay: idx * 0.05 }}
                        className="card"
                    >
                        <div className="flex items-center gap-3">
                            <div className={`p-2.5 rounded-apple ${stat.color}`}>
                                <stat.icon className="w-5 h-5" />
                            </div>
                            <div>
                                <p className="text-xs text-apple-gray-500">{stat.label}</p>
                                <p className="text-xl font-semibold text-apple-gray-900">{stat.value}</p>
                            </div>
                        </div>
                    </motion.div>
                ))}
            </div>

            {/* Quota Management */}
            <div className="card mb-8">
                <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">Quota & Limits</h2>
                <div className="grid grid-cols-2 gap-6">
                    <div>
                        <label className="block text-sm font-medium text-apple-gray-700 mb-1">
                            Monthly Token Limit
                        </label>
                        <p className="text-xs text-apple-gray-400 mb-2">0 = unlimited</p>
                        <input
                            type="number"
                            value={tokenLimit}
                            onChange={(e) => setTokenLimit(e.target.value)}
                            className="input w-full"
                            min={0}
                        />
                        {user.monthly_token_limit > 0 && (
                            <div className="mt-2">
                                <div className="flex justify-between text-xs text-apple-gray-500 mb-1">
                                    <span>{(user.usage_month?.total_tokens || 0).toLocaleString()} used</span>
                                    <span>{user.monthly_token_limit.toLocaleString()} limit</span>
                                </div>
                                <div className="h-2 bg-apple-gray-100 rounded-full overflow-hidden">
                                    <div
                                        className={`h-full rounded-full transition-all ${tokenUsagePct > 90 ? 'bg-red-500' : tokenUsagePct > 70 ? 'bg-amber-500' : 'bg-apple-blue'}`}
                                        style={{ width: `${tokenUsagePct}%` }}
                                    />
                                </div>
                            </div>
                        )}
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-apple-gray-700 mb-1">
                            Monthly Budget (USD)
                        </label>
                        <p className="text-xs text-apple-gray-400 mb-2">0 = unlimited</p>
                        <input
                            type="number"
                            value={budgetLimit}
                            onChange={(e) => setBudgetLimit(e.target.value)}
                            className="input w-full"
                            min={0}
                            step={0.01}
                        />
                        {user.monthly_budget_usd > 0 && (
                            <div className="mt-2">
                                <div className="flex justify-between text-xs text-apple-gray-500 mb-1">
                                    <span>${(user.usage_month?.total_cost || 0).toFixed(4)} used</span>
                                    <span>${user.monthly_budget_usd.toFixed(2)} limit</span>
                                </div>
                                <div className="h-2 bg-apple-gray-100 rounded-full overflow-hidden">
                                    <div
                                        className={`h-full rounded-full transition-all ${budgetUsagePct > 90 ? 'bg-red-500' : budgetUsagePct > 70 ? 'bg-amber-500' : 'bg-green-500'}`}
                                        style={{ width: `${budgetUsagePct}%` }}
                                    />
                                </div>
                            </div>
                        )}
                    </div>
                </div>
                <div className="mt-4 flex justify-end">
                    <button onClick={handleSaveQuota} className="btn-primary px-6">
                        Save Quota
                    </button>
                </div>
            </div>

            {/* Usage Chart */}
            <div className="card mb-8">
                <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">Usage (Last 30 Days)</h2>
                {usage.length > 0 ? (
                    <ResponsiveContainer width="100%" height={280}>
                        <BarChart data={usage}>
                            <CartesianGrid strokeDasharray="3 3" stroke="#E5E5EA" />
                            <XAxis
                                dataKey="date"
                                tick={{ fontSize: 11, fill: '#8E8E93' }}
                                tickFormatter={(v) => new Date(v).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}
                            />
                            <YAxis tick={{ fontSize: 11, fill: '#8E8E93' }} />
                            <Tooltip
                                contentStyle={{ borderRadius: 12, border: '1px solid #E5E5EA', fontSize: 13 }}
                            />
                            <Bar dataKey="requests" name="Requests" fill="#007AFF" radius={[4, 4, 0, 0]} />
                        </BarChart>
                    </ResponsiveContainer>
                ) : (
                    <p className="text-center text-apple-gray-400 py-8">No usage data</p>
                )}
            </div>

            {/* API Keys */}
            <div className="card">
                <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">
                    API Keys ({apiKeys.length})
                </h2>
                {apiKeys.length === 0 ? (
                    <p className="text-center text-apple-gray-400 py-8">No API keys</p>
                ) : (
                    <table className="w-full">
                        <thead>
                            <tr className="border-b border-apple-gray-200">
                                <th className="text-left py-2 px-3 text-sm font-medium text-apple-gray-500">Name</th>
                                <th className="text-left py-2 px-3 text-sm font-medium text-apple-gray-500">Prefix</th>
                                <th className="text-left py-2 px-3 text-sm font-medium text-apple-gray-500">Status</th>
                                <th className="text-left py-2 px-3 text-sm font-medium text-apple-gray-500">Last Used</th>
                                <th className="text-left py-2 px-3 text-sm font-medium text-apple-gray-500">Created</th>
                            </tr>
                        </thead>
                        <tbody>
                            {apiKeys.map((key) => (
                                <tr key={key.id} className="border-b border-apple-gray-100">
                                    <td className="py-2 px-3 text-sm text-apple-gray-900 font-medium">{key.name}</td>
                                    <td className="py-2 px-3 text-sm text-apple-gray-500 font-mono">{key.key_prefix}...</td>
                                    <td className="py-2 px-3">
                                        <span className={`inline-flex px-2 py-0.5 rounded-full text-xs font-medium ${key.is_active ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
                                            {key.is_active ? 'Active' : 'Revoked'}
                                        </span>
                                    </td>
                                    <td className="py-2 px-3 text-sm text-apple-gray-500">
                                        {key.last_used_at ? new Date(key.last_used_at).toLocaleDateString() : 'Never'}
                                    </td>
                                    <td className="py-2 px-3 text-sm text-apple-gray-500">
                                        {new Date(key.created_at).toLocaleDateString()}
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                )}
            </div>
        </div>
    );
}

export default UserDetailPage;

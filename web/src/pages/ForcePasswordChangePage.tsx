import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { motion } from 'framer-motion';
import toast from 'react-hot-toast';
import { userApi, getApiErrorMessage } from '@/lib/api';
import { useAuthStore } from '@/stores/authStore';

function ForcePasswordChangePage() {
    const navigate = useNavigate();
    const { user, updateUser, logout } = useAuthStore();
    const [loading, setLoading] = useState(false);
    const [formData, setFormData] = useState({
        currentPassword: '',
        newPassword: '',
        confirmPassword: '',
    });

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();

        if (!formData.currentPassword || !formData.newPassword || !formData.confirmPassword) {
            toast.error('Please fill in all fields');
            return;
        }

        if (formData.newPassword !== formData.confirmPassword) {
            toast.error('New passwords do not match');
            return;
        }

        if (formData.newPassword.length < 6) {
            toast.error('Password must be at least 6 characters');
            return;
        }

        setLoading(true);
        try {
            await userApi.changePassword({
                old_password: formData.currentPassword,
                new_password: formData.newPassword,
            });

            // Update local state to reflect password change
            if (user) {
                updateUser({ ...user, require_password_change: false });
            }

            toast.success('Password changed successfully');
            navigate('/dashboard');
        } catch (error: unknown) {
            toast.error(getApiErrorMessage(error, 'Failed to change password'));
        } finally {
            setLoading(false);
        }
    };

    const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        setFormData((prev) => ({
            ...prev,
            [e.target.id]: e.target.value,
        }));
    };

    const handleLogout = () => {
        logout();
        navigate('/login');
    };

    return (
        <div className="min-h-screen flex items-center justify-center bg-apple-gray-50 px-4">
            <motion.div
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.4 }}
                className="w-full max-w-md"
            >
                <div className="text-center mb-8">
                    <h1 className="text-3xl font-semibold text-apple-gray-900 mb-2">Security Update</h1>
                    <p className="text-apple-gray-500">Your account requires a password change to continue.</p>
                </div>

                <div className="card">
                    <form onSubmit={handleSubmit} className="space-y-4">
                        <div>
                            <label htmlFor="currentPassword" className="label">
                                Current Password
                            </label>
                            <input
                                type="password"
                                id="currentPassword"
                                value={formData.currentPassword}
                                onChange={handleInputChange}
                                className="input"
                                required
                            />
                        </div>
                        <div>
                            <label htmlFor="newPassword" className="label">
                                New Password
                            </label>
                            <input
                                type="password"
                                id="newPassword"
                                value={formData.newPassword}
                                onChange={handleInputChange}
                                className="input"
                                required
                            />
                        </div>
                        <div>
                            <label htmlFor="confirmPassword" className="label">
                                Confirm New Password
                            </label>
                            <input
                                type="password"
                                id="confirmPassword"
                                value={formData.confirmPassword}
                                onChange={handleInputChange}
                                className="input"
                                required
                            />
                        </div>
                        <div className="pt-4 flex flex-col gap-3">
                            <button
                                type="submit"
                                className="btn btn-primary w-full justify-center"
                                disabled={loading}
                            >
                                {loading ? 'Changing Password...' : 'Change Password'}
                            </button>
                            <button
                                type="button"
                                className="btn btn-secondary w-full justify-center"
                                onClick={handleLogout}
                            >
                                Sign Out
                            </button>
                        </div>
                    </form>
                </div>
            </motion.div>
        </div>
    );
}

export default ForcePasswordChangePage;

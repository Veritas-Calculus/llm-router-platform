import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { motion } from 'framer-motion';
import toast from 'react-hot-toast';
import { useMutation } from '@apollo/client/react';
import { CHANGE_PASSWORD } from '@/lib/graphql/operations';
import { useAuthStore } from '@/stores/authStore';
import { useTranslation } from '@/lib/i18n';

function ForcePasswordChangePage() {
    const { t } = useTranslation();
    const navigate = useNavigate();
    const { user, updateUser, logout } = useAuthStore();
    const [changePwd] = useMutation(CHANGE_PASSWORD);
    const [loading, setLoading] = useState(false);
    const [formData, setFormData] = useState({
        currentPassword: '',
        newPassword: '',
        confirmPassword: '',
    });

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();

        if (!formData.currentPassword || !formData.newPassword || !formData.confirmPassword) {
            toast.error(t('auth.force_change_fill_all'));
            return;
        }

        if (formData.newPassword !== formData.confirmPassword) {
            toast.error(t('auth.force_change_mismatch'));
            return;
        }

        if (formData.newPassword.length < 6) {
            toast.error(t('auth.force_change_min_length'));
            return;
        }

        setLoading(true);
        try {
            await changePwd({
                variables: { input: { oldPassword: formData.currentPassword, newPassword: formData.newPassword } },
            });

            // Update local state to reflect password change
            if (user) {
                updateUser({ ...user, require_password_change: false });
            }

            toast.success(t('auth.force_change_success'));
            navigate('/dashboard');
        } catch {
            toast.error(t('auth.force_change_error'));
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
                    <h1 className="text-3xl font-semibold text-apple-gray-900 mb-2">{t('auth.force_change_heading')}</h1>
                    <p className="text-apple-gray-500">{t('auth.force_change_subheading')}</p>
                </div>

                <div className="card">
                    <form onSubmit={handleSubmit} className="space-y-4">
                        <div>
                            <label htmlFor="currentPassword" className="label">
                                {t('auth.force_change_current')}
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
                                {t('auth.force_change_new')}
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
                                {t('auth.force_change_confirm')}
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
                                {loading ? t('auth.force_change_in_progress') : t('auth.force_change_btn')}
                            </button>
                            <button
                                type="button"
                                className="btn btn-secondary w-full justify-center"
                                onClick={handleLogout}
                            >
                                {t('auth.logout')}
                            </button>
                        </div>
                    </form>
                </div>
            </motion.div>
        </div>
    );
}

export default ForcePasswordChangePage;

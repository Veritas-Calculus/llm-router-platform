/* eslint-disable @typescript-eslint/no-explicit-any */
 
import { useState } from 'react';
import { motion } from 'framer-motion';
import toast from 'react-hot-toast';
import { useAuthStore } from '@/stores/authStore';
import { useMutation } from '@apollo/client/react';
import { CHANGE_PASSWORD, GENERATE_MFA_SECRET, VERIFY_AND_ENABLE_MFA, DISABLE_MFA, UPDATE_PROFILE } from '@/lib/graphql/operations';
import { ShieldCheckIcon, ShieldExclamationIcon, DocumentDuplicateIcon } from '@heroicons/react/24/outline';
import { QRCodeSVG } from 'qrcode.react';
import { useTranslation } from '@/lib/i18n';

function SettingsPage() {
  const { t } = useTranslation();
  const { user, updateUser } = useAuthStore();
  const [changePwd] = useMutation<any>(CHANGE_PASSWORD);
  const [generateMfaSecret] = useMutation<any>(GENERATE_MFA_SECRET);
  const [verifyAndEnableMfa] = useMutation<any>(VERIFY_AND_ENABLE_MFA);
  const [disableMfaMut] = useMutation<any>(DISABLE_MFA);
  const [updateProfile] = useMutation<any>(UPDATE_PROFILE);

  // MFA Setup State
  const [isMfaModalOpen, setIsMfaModalOpen] = useState(false);
  const [mfaSecretData, setMfaSecretData] = useState<{ secret: string; qrCodeUrl: string; backupCodes: string[] } | null>(null);
  const [verificationCode, setVerificationCode] = useState('');
  
  // MFA Disable State
  const [isDisableMfaModalOpen, setIsDisableMfaModalOpen] = useState(false);
  const [disableCode, setDisableCode] = useState('');
  const [saving, setSaving] = useState(false);
  const [formData, setFormData] = useState({
    name: user?.name || '',
    email: user?.email || '',
    currentPassword: '',
    newPassword: '',
    confirmPassword: '',
  });

  const handleSaveProfile = async () => {
    setSaving(true);
    try {
      const { data: result } = await updateProfile({
        variables: { input: { name: formData.name } },
      });
      if (result?.updateProfile && user) {
        updateUser({ ...user, name: result.updateProfile.name });
      }
      toast.success(t('settings.profile_updated'));
    } catch {
      toast.error(t('settings.profile_update_error'));
    } finally {
      setSaving(false);
    }
  };

  const handleChangePassword = async () => {
    if (!formData.currentPassword || !formData.newPassword) {
      toast.error(t('settings.password_fill_all'));
      return;
    }

    if (formData.newPassword !== formData.confirmPassword) {
      toast.error(t('settings.password_mismatch'));
      return;
    }

    if (formData.newPassword.length < 6) {
      toast.error(t('settings.password_min_length'));
      return;
    }

    setSaving(true);
    try {
      await changePwd({
        variables: { input: { oldPassword: formData.currentPassword, newPassword: formData.newPassword } },
      });
      setFormData((prev) => ({
        ...prev,
        currentPassword: '',
        newPassword: '',
        confirmPassword: '',
      }));
      toast.success(t('settings.change_password_success'));
    } catch {
      toast.error(t('settings.change_password_error'));
    } finally {
      setSaving(false);
    }
  };

  const handleEnableMfaClick = async () => {
    setSaving(true);
    try {
      const { data } = await generateMfaSecret();
      if (data?.generateMfaSecret) {
        setMfaSecretData(data.generateMfaSecret);
        setIsMfaModalOpen(true);
      }
    } catch (err: any) {
      toast.error(err.message || 'Failed to generate MFA setup');
    } finally {
      setSaving(false);
    }
  };

  const handleVerifyAndEnableMfa = async () => {
    if (!verificationCode || verificationCode.length !== 6) {
      toast.error(t('settings.mfa_invalid_code'));
      return;
    }
    setSaving(true);
    try {
      await verifyAndEnableMfa({ variables: { code: verificationCode } });
      toast.success(t('settings.mfa_enabled_success'));
      setIsMfaModalOpen(false);
      setMfaSecretData(null);
      setVerificationCode('');
      if (user) {
        updateUser({ ...user, mfaEnabled: true });
      }
    } catch (err: any) {
      toast.error(err.message || 'MFA verification failed. Please check the code.');
    } finally {
      setSaving(false);
    }
  };

  const handleDisableMfa = async () => {
    if (!disableCode || disableCode.length !== 6) {
      toast.error(t('settings.mfa_invalid_code'));
      return;
    }
    setSaving(true);
    try {
      await disableMfaMut({ variables: { code: disableCode } });
      toast.success(t('settings.mfa_disabled_success'));
      setIsDisableMfaModalOpen(false);
      setDisableCode('');
      if (user) {
        updateUser({ ...user, mfaEnabled: false });
      }
    } catch (err: any) {
      toast.error(err.message || 'Failed to disable MFA. Invalid code.');
    } finally {
      setSaving(false);
    }
  };

  const copyToClipboard = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      toast.success(t('common.copied_clipboard'));
    } catch {
      toast.error(t('common.copy_failed'));
    }
  };

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-semibold text-apple-gray-900">{t('settings.title')}</h1>
        <p className="text-apple-gray-500 mt-1">{t('settings.subtitle')}</p>
      </div>

      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        className="card max-w-2xl"
      >
        <h2 className="text-lg font-semibold text-apple-gray-900 mb-6">{t('settings.profile')}</h2>
        <div className="space-y-4">
          <div>
            <label htmlFor="name" className="label">
              {t('common.name')}
            </label>
            <input
              type="text"
              id="name"
              value={formData.name}
              onChange={(e) =>
                setFormData((prev) => ({ ...prev, name: e.target.value }))
              }
              className="input"
            />
          </div>
          <div>
            <label htmlFor="email" className="label">
              {t('auth.email')}
            </label>
            <input
              type="email"
              id="email"
              value={formData.email}
              onChange={(e) =>
                setFormData((prev) => ({ ...prev, email: e.target.value }))
              }
              className="input"
              disabled
            />
            <p className="text-xs text-apple-gray-500 mt-1">
              {t('settings.email_hint')}
            </p>
          </div>
          <div className="pt-4">
            <button
              onClick={handleSaveProfile}
              className="btn btn-primary"
              disabled={saving}
            >
              {saving ? t('common.saving') : t('settings.save_changes')}
            </button>
          </div>
        </div>
      </motion.div>

      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.1 }}
        className="card max-w-2xl"
      >
        <h2 className="text-lg font-semibold text-apple-gray-900 mb-6">{t('settings.change_password')}</h2>
        <div className="space-y-4">
          <div>
            <label htmlFor="currentPassword" className="label">
              {t('settings.current_password')}
            </label>
            <input
              type="password"
              id="currentPassword"
              value={formData.currentPassword}
              onChange={(e) =>
                setFormData((prev) => ({ ...prev, currentPassword: e.target.value }))
              }
              className="input"
            />
          </div>
          <div>
            <label htmlFor="newPassword" className="label">
              {t('settings.new_password')}
            </label>
            <input
              type="password"
              id="newPassword"
              value={formData.newPassword}
              onChange={(e) =>
                setFormData((prev) => ({ ...prev, newPassword: e.target.value }))
              }
              className="input"
            />
          </div>
          <div>
            <label htmlFor="confirmPassword" className="label">
              {t('settings.confirm_new_password')}
            </label>
            <input
              type="password"
              id="confirmPassword"
              value={formData.confirmPassword}
              onChange={(e) =>
                setFormData((prev) => ({ ...prev, confirmPassword: e.target.value }))
              }
              className="input"
            />
          </div>
          <div className="pt-4">
            <button
              onClick={handleChangePassword}
              className="btn btn-primary"
              disabled={saving}
            >
              {saving ? t('settings.changing') : t('settings.change_password')}
            </button>
          </div>
        </div>
      </motion.div>

      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.15 }}
        className="card max-w-2xl"
      >
        <div className="flex items-center gap-3 mb-4">
          <div className={`p-2 rounded-full ${user?.mfaEnabled ? 'bg-apple-green/10 text-apple-green' : 'bg-apple-gray-100 text-apple-gray-500'}`}>
            {user?.mfaEnabled ? <ShieldCheckIcon className="w-6 h-6" /> : <ShieldExclamationIcon className="w-6 h-6" />}
          </div>
          <div>
            <h2 className="text-lg font-semibold text-apple-gray-900">{t('settings.mfa_title')}</h2>
            <p className="text-sm text-apple-gray-500">
              {user?.mfaEnabled ? t('settings.mfa_enabled_desc') : t('settings.mfa_disabled_desc')}
            </p>
          </div>
        </div>

        <div className="pt-2 border-t border-apple-gray-100 mt-4">
          {user?.mfaEnabled ? (
            <button
              onClick={() => setIsDisableMfaModalOpen(true)}
              className="btn mt-4 bg-apple-gray-100 text-apple-gray-700 hover:bg-apple-gray-200"
              disabled={saving}
            >
              {t('settings.mfa_disable')}
            </button>
          ) : (
            <button
              onClick={handleEnableMfaClick}
              className="btn btn-primary mt-4"
              disabled={saving}
            >
              {saving ? t('settings.mfa_loading') : t('settings.mfa_setup')}
            </button>
          )}
        </div>
      </motion.div>

      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.2 }}
        className="card max-w-2xl"
      >
        <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">{t('settings.danger_zone')}</h2>
        <p className="text-apple-gray-500 mb-4">
          {t('settings.danger_zone_desc')}
        </p>
        <button
          onClick={() => toast.error(t('settings.delete_account_disabled'))}
          className="btn btn-danger"
        >
          {t('settings.delete_account')}
        </button>
      </motion.div>

      {/* MFA Setup Modal */}
      {isMfaModalOpen && mfaSecretData && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <motion.div
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            className="bg-[var(--theme-bg-card)] rounded-apple-lg shadow-apple-xl p-6 w-full max-w-lg mx-4"
          >
            <h3 className="text-xl font-semibold text-apple-gray-900 mb-6">{t('settings.mfa_setup_title')}</h3>
            
            <div className="space-y-6">
              <div className="flex flex-col items-center justify-center space-y-4">
                <p className="text-sm text-apple-gray-600 text-center">
                  {t('settings.mfa_scan_desc')}
                </p>
                <div className="p-4 bg-white rounded-xl border border-apple-gray-200 shadow-sm inline-block">
                  <QRCodeSVG value={mfaSecretData.qrCodeUrl} size={192} level="M" />
                </div>
                <div className="flex items-center gap-2 mt-2">
                  <span className="text-xs text-apple-gray-500">{t('settings.mfa_manual_key')}</span>
                  <code className="text-sm font-mono bg-apple-gray-50 px-2 py-1 rounded">{mfaSecretData.secret}</code>
                  <button onClick={() => copyToClipboard(mfaSecretData.secret)} className="text-apple-gray-400 hover:text-apple-blue transition-colors">
                    <DocumentDuplicateIcon className="w-4 h-4" />
                  </button>
                </div>
              </div>

              <div className="border-t border-apple-gray-100 pt-6">
                <h4 className="text-sm font-semibold text-apple-gray-900 mb-2 flex items-center gap-2">
                  <ShieldExclamationIcon className="w-4 h-4 text-apple-orange" />
                  {t('settings.mfa_recovery')}
                </h4>
                <p className="text-xs text-apple-gray-500 mb-3">
                  {t('settings.mfa_recovery_desc')}
                </p>
                <div className="bg-apple-gray-50 p-4 rounded-apple border border-apple-gray-200 flex flex-wrap gap-2 text-sm font-mono text-apple-gray-700">
                  {mfaSecretData.backupCodes.map((code, i) => (
                    <span key={i} className="bg-white px-2 py-1 rounded border border-apple-gray-100 shadow-sm">
                      {code}
                    </span>
                  ))}
                </div>
              </div>

              <div className="border-t border-apple-gray-100 pt-6">
                <label className="block text-sm font-medium text-apple-gray-700 mb-2">
                  {t('settings.mfa_verify_setup')}
                </label>
                <p className="text-xs text-apple-gray-500 mb-3">
                  {t('settings.mfa_verify_desc')}
                </p>
                <input
                  type="text"
                  maxLength={6}
                  value={verificationCode}
                  onChange={(e) => setVerificationCode(e.target.value.replace(/[^0-9]/g, ''))}
                  placeholder="000000"
                  className="input tracking-[0.5em] font-mono text-center text-lg w-full max-w-[200px] mx-auto block"
                />
              </div>
            </div>

            <div className="flex justify-end gap-3 mt-8">
              <button
                onClick={() => {
                  setIsMfaModalOpen(false);
                  setMfaSecretData(null);
                  setVerificationCode('');
                }}
                className="btn btn-secondary"
                disabled={saving}
              >
                {t('common.cancel')}
              </button>
              <button
                onClick={handleVerifyAndEnableMfa}
                className="btn btn-primary"
                disabled={saving || verificationCode.length !== 6}
              >
                {saving ? t('settings.mfa_verifying') : t('settings.mfa_verify_enable')}
              </button>
            </div>
          </motion.div>
        </div>
      )}

      {/* Disable MFA Modal */}
      {isDisableMfaModalOpen && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <motion.div
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            className="bg-[var(--theme-bg-card)] rounded-apple-lg shadow-apple-xl p-6 w-full max-w-sm mx-4"
          >
            <h3 className="text-xl font-semibold text-apple-gray-900 mb-4">{t('settings.mfa_disable_title')}</h3>
            
            <p className="text-sm text-apple-gray-600 mb-6">
              {t('settings.mfa_disable_desc')}
            </p>

            <div className="mb-6">
              <label className="block text-sm font-medium text-apple-gray-700 mb-2">
                {t('settings.mfa_auth_code')}
              </label>
              <input
                type="text"
                maxLength={6}
                value={disableCode}
                onChange={(e) => setDisableCode(e.target.value.replace(/[^0-9]/g, ''))}
                placeholder="000000"
                className="input tracking-[0.5em] font-mono text-center text-lg w-full block"
              />
            </div>

            <div className="flex justify-end gap-3">
              <button
                onClick={() => {
                  setIsDisableMfaModalOpen(false);
                  setDisableCode('');
                }}
                className="btn btn-secondary"
                disabled={saving}
              >
                {t('common.cancel')}
              </button>
              <button
                onClick={handleDisableMfa}
                className="btn btn-danger"
                disabled={saving || disableCode.length !== 6}
              >
                {saving ? t('settings.mfa_disabling') : t('settings.mfa_disable')}
              </button>
            </div>
          </motion.div>
        </div>
      )}
    </div>
  );
}

export default SettingsPage;

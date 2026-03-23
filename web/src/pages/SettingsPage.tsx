/* eslint-disable @typescript-eslint/no-explicit-any */
 
import { useState } from 'react';
import { motion } from 'framer-motion';
import toast from 'react-hot-toast';
import { useAuthStore } from '@/stores/authStore';
import { useMutation } from '@apollo/client/react';
import { CHANGE_PASSWORD, GENERATE_MFA_SECRET, VERIFY_AND_ENABLE_MFA, DISABLE_MFA } from '@/lib/graphql/operations';
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
      await new Promise((resolve) => setTimeout(resolve, 1000));
      toast.success('Profile updated');
    } catch {
      toast.error('Failed to update profile');
    } finally {
      setSaving(false);
    }
  };

  const handleChangePassword = async () => {
    if (!formData.currentPassword || !formData.newPassword) {
      toast.error('Please fill in all password fields');
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
      toast.success('Password changed');
    } catch {
      toast.error('Failed to change password');
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
      toast.error('Please enter a valid 6-digit code');
      return;
    }
    setSaving(true);
    try {
      await verifyAndEnableMfa({ variables: { code: verificationCode } });
      toast.success('Two-factor authentication enabled successfully.');
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
      toast.error('Please enter a valid 6-digit code');
      return;
    }
    setSaving(true);
    try {
      await disableMfaMut({ variables: { code: disableCode } });
      toast.success('Two-factor authentication disabled.');
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
      toast.success('Copied to clipboard');
    } catch {
      toast.error('Failed to copy');
    }
  };

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-semibold text-apple-gray-900">Settings</h1>
        <p className="text-apple-gray-500 mt-1">Manage your account settings</p>
      </div>

      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        className="card max-w-2xl"
      >
        <h2 className="text-lg font-semibold text-apple-gray-900 mb-6">Profile</h2>
        <div className="space-y-4">
          <div>
            <label htmlFor="name" className="label">
              Name
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
              Email
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
              Contact support to change your email
            </p>
          </div>
          <div className="pt-4">
            <button
              onClick={handleSaveProfile}
              className="btn btn-primary"
              disabled={saving}
            >
              {saving ? 'Saving...' : 'Save Changes'}
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
        <h2 className="text-lg font-semibold text-apple-gray-900 mb-6">Change Password</h2>
        <div className="space-y-4">
          <div>
            <label htmlFor="currentPassword" className="label">
              Current Password
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
              New Password
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
              Confirm New Password
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
              {saving ? 'Changing...' : 'Change Password'}
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
            <h2 className="text-lg font-semibold text-apple-gray-900">Two-Factor Authentication (2FA)</h2>
            <p className="text-sm text-apple-gray-500">
              {user?.mfaEnabled ? 'Your account is protected by TOTP-based 2FA.' : 'Add an extra layer of security to your account.'}
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
              Disable 2FA
            </button>
          ) : (
            <button
              onClick={handleEnableMfaClick}
              className="btn btn-primary mt-4"
              disabled={saving}
            >
              {saving ? 'Loading...' : 'Set Up 2FA'}
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
        <h2 className="text-lg font-semibold text-apple-gray-900 mb-4">Danger Zone</h2>
        <p className="text-apple-gray-500 mb-4">
          Once you delete your account, there is no going back. Please be certain.
        </p>
        <button
          onClick={() => toast.error('Account deletion is disabled')}
          className="btn btn-danger"
        >
          Delete Account
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
            <h3 className="text-xl font-semibold text-apple-gray-900 mb-6">Set Up Two-Factor Authentication</h3>
            
            <div className="space-y-6">
              <div className="flex flex-col items-center justify-center space-y-4">
                <p className="text-sm text-apple-gray-600 text-center">
                  Scan this QR code with your authenticator app (e.g., Google Authenticator, Authy).
                </p>
                <div className="p-4 bg-white rounded-xl border border-apple-gray-200 shadow-sm inline-block">
                  <QRCodeSVG value={mfaSecretData.qrCodeUrl} size={192} level="M" />
                </div>
                <div className="flex items-center gap-2 mt-2">
                  <span className="text-xs text-apple-gray-500">Manual Entry Key:</span>
                  <code className="text-sm font-mono bg-apple-gray-50 px-2 py-1 rounded">{mfaSecretData.secret}</code>
                  <button onClick={() => copyToClipboard(mfaSecretData.secret)} className="text-apple-gray-400 hover:text-apple-blue transition-colors">
                    <DocumentDuplicateIcon className="w-4 h-4" />
                  </button>
                </div>
              </div>

              <div className="border-t border-apple-gray-100 pt-6">
                <h4 className="text-sm font-semibold text-apple-gray-900 mb-2 flex items-center gap-2">
                  <ShieldExclamationIcon className="w-4 h-4 text-apple-orange" />
                  Recovery Codes
                </h4>
                <p className="text-xs text-apple-gray-500 mb-3">
                  Save these recovery codes in a secure place. You can use them to sign in if you lose access to your authenticator app.
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
                  Verify Setup
                </label>
                <p className="text-xs text-apple-gray-500 mb-3">
                  Enter the 6-digit code from your authenticator app to complete setup.
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
                Cancel
              </button>
              <button
                onClick={handleVerifyAndEnableMfa}
                className="btn btn-primary"
                disabled={saving || verificationCode.length !== 6}
              >
                {saving ? 'Verifying...' : 'Verify & Enable'}
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
            <h3 className="text-xl font-semibold text-apple-gray-900 mb-4">Disable 2FA</h3>
            
            <p className="text-sm text-apple-gray-600 mb-6">
              To disable Two-Factor Authentication, please enter a code from your authenticator app or one of your recovery codes.
            </p>

            <div className="mb-6">
              <label className="block text-sm font-medium text-apple-gray-700 mb-2">
                Authentication Code
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
                Cancel
              </button>
              <button
                onClick={handleDisableMfa}
                className="btn btn-danger"
                disabled={saving || disableCode.length !== 6}
              >
                {saving ? 'Disabling...' : 'Disable 2FA'}
              </button>
            </div>
          </motion.div>
        </div>
      )}
    </div>
  );
}

export default SettingsPage;

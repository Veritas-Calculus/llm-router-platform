/* eslint-disable @typescript-eslint/no-explicit-any */
 
import { useState, useEffect, useRef } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { motion } from 'framer-motion';
import toast from 'react-hot-toast';
import { useMutation, useQuery } from '@apollo/client/react';
import { LOGIN, REGISTER, REGISTRATION_MODE } from '@/lib/graphql/operations';
import { useAuthStore } from '@/stores/authStore';
import { Turnstile, type TurnstileInstance } from '@marsidev/react-turnstile';
import { useTranslation } from '@/lib/i18n';

function LoginPage() {
  const navigate = useNavigate();
  const setAuth = useAuthStore((state) => state.setAuth);
  const { t } = useTranslation();
  const [loginMut] = useMutation(LOGIN);
  const [registerMut] = useMutation(REGISTER);
  const [isLogin, setIsLogin] = useState(true);
  const [isSsoMode, setIsSsoMode] = useState(false);
  const [loading, setLoading] = useState(false);
  const [formData, setFormData] = useState({
    email: '',
    password: '',
    name: '',
    inviteCode: '',
  });

  // Turnstile CAPTCHA state
  const [captchaToken, setCaptchaToken] = useState<string | null>(null);
  const [captchaConfig, setCaptchaConfig] = useState<{ enabled: boolean; siteKey: string }>({ enabled: false, siteKey: '' });
  const turnstileRef = useRef<TurnstileInstance | null>(null);

  // Fetch Turnstile config from backend
  useEffect(() => {
    fetch('/api/v1/captcha/config')
      .then(r => r.json())
      .then(data => setCaptchaConfig({ enabled: data.enabled, siteKey: data.siteKey || '' }))
      .catch(() => {});
  }, []);

  // Query registration mode (public, no auth required)
  const { data: regModeData } = useQuery<{ registrationMode: { mode: string; inviteCodeRequired: boolean } }>(REGISTRATION_MODE, { fetchPolicy: 'cache-first' });
  const regMode = regModeData?.registrationMode?.mode ?? 'closed';
  const inviteRequired = regModeData?.registrationMode?.inviteCodeRequired ?? false;
  const registrationOpen = regMode === 'open' || regMode === 'invite';

  // Fetch available OAuth2 providers
  const [oauthProviders, setOauthProviders] = useState<Array<{ id: string; name: string }>>([]);
  useEffect(() => {
    fetch('/auth/oauth2/providers')
      .then(r => r.json())
      .then(data => setOauthProviders(data.providers || []))
      .catch(() => {});
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    // CAPTCHA validation
    if (captchaConfig.enabled && !captchaToken) {
      toast.error(t('auth.captcha_required'));
      return;
    }

    setLoading(true);

    try {
      if (isLogin) {
        const { data } = await loginMut({
          variables: { input: { email: formData.email, password: formData.password, captchaToken } },
        });
        const resp = (data as any)?.login;
        setAuth(resp.token, resp.user);
        toast.success(t('auth.welcome_back'));
      } else {
        const registerInput: Record<string, string | null> = {
          email: formData.email,
          password: formData.password,
          name: formData.name,
          captchaToken,
        };
        if (inviteRequired && formData.inviteCode) {
          registerInput.inviteCode = formData.inviteCode;
        }
        const { data } = await registerMut({
          variables: { input: registerInput },
        });
        const resp = (data as any)?.register;
        setAuth(resp.token, resp.user);
        toast.success(t('auth.account_created'));
      }
      // Read from store (just set above) to determine where to navigate
      const user = useAuthStore.getState().user;
      navigate(user?.require_password_change ? '/change-password' : '/dashboard');
    } catch {
      toast.error(isLogin ? t('auth.invalid_credentials') : t('auth.registration_failed'));
      // Reset turnstile widget on failure so user can retry
      turnstileRef.current?.reset();
      setCaptchaToken(null);
    } finally {
      setLoading(false);
    }
  };

  const handleSsoSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!formData.email) {
      toast.error(t('auth.sso_email_required'));
      return;
    }
    setLoading(true);
    try {
      const res = await fetch('/api/v1/sso/discover', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: formData.email })
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || t('auth.sso_failed'));
      window.location.href = data.redirect_url;
    } catch (err: any) {
      toast.error(err.message || t('auth.sso_failed'));
      setLoading(false);
    }
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setFormData((prev) => ({
      ...prev,
      [e.target.name]: e.target.value,
    }));
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-apple-gray-50 px-4">
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4 }}
        className="w-full max-w-md"
      >
        {/* Brand */}
        <div className="text-center mb-10">
          <div className="w-16 h-16 bg-gradient-to-br from-apple-blue to-blue-600 rounded-2xl flex items-center justify-center shadow-lg mx-auto mb-5">
            <span className="text-white font-bold text-2xl">R</span>
          </div>
          <h1 className="text-3xl font-semibold text-apple-gray-900 mb-2">{t('auth.platform_name')}</h1>
          <p className="text-apple-gray-500">{t('auth.platform_slogan')}</p>
        </div>

        <div className="card">
          {/* Segmented Control */}
          <div className="flex bg-apple-gray-100 rounded-xl p-1 mb-8 border border-apple-gray-200">
            <button
              onClick={() => setIsLogin(true)}
              className={`flex-1 py-2 text-sm font-semibold rounded-lg transition-all duration-200 ${
                isLogin
                  ? 'bg-white text-apple-blue shadow-sm border border-apple-gray-200'
                  : 'text-apple-gray-500 hover:text-apple-gray-700'
              }`}
            >
              {t('auth.login')}
            </button>
            <button
              onClick={() => setIsLogin(false)}
              disabled={!registrationOpen}
              className={`flex-1 py-2 text-sm font-semibold rounded-lg transition-all duration-200 ${
                !isLogin
                  ? 'bg-white text-apple-blue shadow-sm border border-apple-gray-200'
                  : !registrationOpen
                    ? 'text-apple-gray-300 cursor-not-allowed'
                    : 'text-apple-gray-500 hover:text-apple-gray-700'
              }`}
              title={!registrationOpen ? t('auth.registration_closed_hint') : undefined}
            >
              {t('auth.register')}
            </button>
          </div>

          {/* Registration closed hint */}
          {!isLogin && !registrationOpen && (
            <div className="mb-6 p-3 bg-amber-50 border border-amber-200 rounded-xl text-sm text-amber-700">
              {t('auth.registration_closed')}
            </div>
          )}

          <form onSubmit={isSsoMode ? handleSsoSubmit : handleSubmit} className="space-y-5">
            {!isLogin && !isSsoMode && (
              <div>
                <label htmlFor="name" className="label">
                  {t('auth.name_label')}
                </label>
                <input
                  type="text"
                  id="name"
                  name="name"
                  value={formData.name}
                  onChange={handleInputChange}
                  className="input"
                  placeholder={t('auth.enter_name')}
                  required={!isLogin}
                />
              </div>
            )}

            <div>
              <label htmlFor="email" className="label">
                {t('auth.email')}
              </label>
              <input
                type="email"
                id="email"
                name="email"
                value={formData.email}
                onChange={handleInputChange}
                className="input"
                placeholder={t('auth.enter_email')}
                required
              />
            </div>

            {!isSsoMode && (
              <>
                <div>
                  <label htmlFor="password" className="label">
                    {t('auth.password')}
              </label>
              <input
                type="password"
                id="password"
                name="password"
                value={formData.password}
                onChange={handleInputChange}
                className="input"
                placeholder={t('auth.enter_password')}
                required
                minLength={6}
              />
              {isLogin && (
                <div className="flex justify-end mt-1.5">
                  <Link
                    to="/forgot-password"
                    className="text-sm text-apple-blue hover:underline font-medium"
                  >
                    {t('auth.forgot_password')}
                  </Link>
                  </div>
                )}
              </div>
              </>
            )}

            {/* Invite Code — only shown in invite mode during registration */}
            {!isLogin && inviteRequired && (
              <div>
                <label htmlFor="inviteCode" className="label">
                  {t('auth.invited_code')}
                </label>
                <input
                  type="text"
                  id="inviteCode"
                  name="inviteCode"
                  value={formData.inviteCode}
                  onChange={handleInputChange}
                  className="input"
                  placeholder={t('auth.enter_invite_code')}
                  required
                />
              </div>
            )}

            {/* Cloudflare Turnstile CAPTCHA */}
            {captchaConfig.enabled && captchaConfig.siteKey && !isSsoMode && (
              <div className="flex justify-center">
                <Turnstile
                  ref={turnstileRef}
                  siteKey={captchaConfig.siteKey}
                  onSuccess={(token) => setCaptchaToken(token)}
                  onExpire={() => setCaptchaToken(null)}
                  onError={() => setCaptchaToken(null)}
                  options={{ theme: 'light', size: 'normal' }}
                />
              </div>
            )}

            <button type="submit" className="btn-primary w-full py-3 rounded-xl text-base font-semibold mt-2" disabled={loading || (!isLogin && !registrationOpen) || (captchaConfig.enabled && !captchaToken && !isSsoMode)}>
              {loading ? (
                <span className="flex items-center justify-center gap-2">
                  <svg className="animate-spin h-5 w-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                  </svg>
                  {t('common.processing')}
                </span>
              ) : isLogin ? (
                t('auth.login')
              ) : (
                t('auth.create_account_btn')
              )}
            </button>
          </form>

          {/* Social Login / SSO options form toggle */}
          {isLogin && (
            <div className="mt-4 text-center">
              <button
                type="button"
                onClick={() => setIsSsoMode(!isSsoMode)}
                className="text-sm font-medium text-apple-gray-500 hover:text-apple-gray-800 transition-colors"
              >
                {isSsoMode ? t('auth.sso_back') : t('auth.sso_login')}
              </button>
            </div>
          )}

          {/* Social Login */}
          {oauthProviders.length > 0 && (
            <>
              <div className="relative my-6">
                <div className="absolute inset-0 flex items-center">
                  <div className="w-full border-t border-apple-gray-200" />
                </div>
                <div className="relative flex justify-center text-xs">
                  <span className="bg-white px-3 text-apple-gray-400 font-medium">{t('auth.or_continue_with')}</span>
                </div>
              </div>
              <div className="flex gap-3">
                {oauthProviders.map(p => (
                  <a
                    key={p.id}
                    href={`/auth/oauth2/${p.id}/redirect`}
                    className="flex-1 flex items-center justify-center gap-2 py-2.5 rounded-xl border border-apple-gray-200 text-sm font-semibold text-apple-gray-700 hover:bg-apple-gray-50 transition-colors"
                  >
                    {p.id === 'github' && (
                      <svg className="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
                        <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
                      </svg>
                    )}
                    {p.id === 'google' && (
                      <svg className="w-5 h-5" viewBox="0 0 24 24">
                        <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" fill="#4285F4" />
                        <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853" />
                        <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05" />
                        <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335" />
                      </svg>
                    )}
                    {p.name}
                  </a>
                ))}
              </div>
            </>
          )}
        </div>
      </motion.div>
    </div>
  );
}

export default LoginPage;

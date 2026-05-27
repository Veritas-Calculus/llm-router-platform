/* eslint-disable @typescript-eslint/no-explicit-any */
 
import { useState, useEffect, useRef, useCallback } from 'react';
import { useNavigate, Link, useLocation } from 'react-router-dom';
import { motion } from 'framer-motion';
import toast from 'react-hot-toast';
import { useMutation, useQuery } from '@apollo/client/react';
import { CAPTCHA_CONFIG, DISCOVER_SSO, LOGIN, PASSWORD_POLICY, REGISTER, REGISTRATION_MODE, SITE_CONFIG_QUERY } from '@/lib/graphql/operations';
import { useAuthStore } from '@/stores/authStore';
import { Turnstile, type TurnstileInstance } from '@marsidev/react-turnstile';
import { useTranslation } from '@/lib/i18n';
import { DEFAULT_PASSWORD_POLICY, type PasswordPolicy as PasswordPolicyType, describePolicy, validatePassword } from '@/lib/auth/passwordPolicy';
import { getRuntimeConfig } from '@/lib/runtime-config';

type LoginLocationState = {
  from?: {
    pathname?: string;
    search?: string;
    hash?: string;
  };
};

type AuthRedirectUser = {
  require_password_change?: boolean | null;
  requirePasswordChange?: boolean | null;
};

// Codes mirror the server-side classifier (handler.go classifyClientError).
type AuthErrorCode =
  | 'UNAUTHENTICATED'
  | 'FORBIDDEN'
  | 'RATE_LIMITED'
  | 'INVALID_CREDENTIALS'
  | 'NOT_FOUND'
  | 'INSUFFICIENT_BALANCE'
  | 'ACCOUNT_DISABLED'
  | 'VALIDATION'
  | 'INTERNAL'
  | undefined;

// A flattened, structured view of a GraphQL error suitable for branching in
// the form. We keep the raw extensions in case a caller needs more, but the
// top-level fields are what the LoginPage actually switches on.
interface NormalizedGqlError {
  message: string;
  code: AuthErrorCode;
  /** extensions.field set by the server VALIDATION classifier; e.g. "password". */
  field?: string;
  extensions?: Record<string, unknown>;
}

class AuthError extends Error {
  readonly code: AuthErrorCode;
  /** All errors returned by the failing mutation, normalized for inline display. */
  readonly errors: NormalizedGqlError[];

  constructor(message: string, code?: AuthErrorCode, errors?: NormalizedGqlError[]) {
    super(message);
    this.code = code;
    this.errors = errors ?? [];
  }
}

// apolloErrorToAuthError preserves extensions.code + extensions.field through
// the Error throw so the catch site can both branch on the stable code AND
// reach inline per-field validation messages without re-parsing the wire data.
function apolloErrorToAuthError(err: { message: string; errors?: ReadonlyArray<{ message: string; extensions?: Record<string, unknown> }> }): AuthError {
  const normalized: NormalizedGqlError[] = (err.errors ?? []).map((e) => {
    const ext = e.extensions ?? {};
    const rawCode = ext.code;
    const code = typeof rawCode === 'string' ? (rawCode as AuthErrorCode) : undefined;
    const rawField = ext.field;
    const field = typeof rawField === 'string' ? rawField : undefined;
    return { message: e.message, code, field, extensions: ext };
  });
  const first = normalized[0];
  return new AuthError(first?.message ?? err.message, first?.code, normalized);
}

function LoginPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const setAuth = useAuthStore((state) => state.setAuth);
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated);
  const authUser = useAuthStore((state) => state.user);
  const { t } = useTranslation();
  const [loginMut] = useMutation(LOGIN);
  const [registerMut] = useMutation(REGISTER);
  const [discoverSsoMut] = useMutation(DISCOVER_SSO);
  const [isLogin, setIsLogin] = useState(true);
  const [isSsoMode, setIsSsoMode] = useState(false);
  const [loading, setLoading] = useState(false);
  const [formData, setFormData] = useState({
    email: '',
    password: '',
    confirmPassword: '',
    name: '',
    inviteCode: '',
  });

  // Field-keyed inline errors, populated by the VALIDATION handler in
  // handleSubmit. Keys mirror server extensions.field values: "email" /
  // "password" / "confirmPassword" / "name" / "inviteCode" / "captchaToken".
  // We reset the whole map on every submission attempt so a successful
  // resubmit doesn't leave stale red text around. H-04.
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  // Captcha state. captchaToken is whatever the active backend produced —
  // the literal dev bypass for ProviderDev, the widget-issued token for
  // turnstile / hcaptcha, or null when the backend is disabled. The signup
  // form refuses to submit until this is set.
  const [captchaToken, setCaptchaToken] = useState<string | null>(null);
  const turnstileRef = useRef<TurnstileInstance | null>(null);

  // Query registration mode (public, no auth required)
  const { data: regModeData } = useQuery<{ registrationMode: { mode: string; inviteCodeRequired: boolean } }>(REGISTRATION_MODE, { fetchPolicy: 'cache-first' });
  const { data: captchaData } = useQuery<{ captchaConfig: { enabled: boolean; siteKey: string } }>(CAPTCHA_CONFIG, { fetchPolicy: 'cache-first' });
  const { data: siteData } = useQuery<{ siteConfig: { siteName: string; subtitle?: string | null; logoUrl?: string | null } }>(SITE_CONFIG_QUERY, { fetchPolicy: 'cache-first' });
  const { data: policyData } = useQuery<{ passwordPolicy: PasswordPolicyType }>(PASSWORD_POLICY, { fetchPolicy: 'cache-first' });
  const passwordPolicy: PasswordPolicyType = policyData?.passwordPolicy ?? DEFAULT_PASSWORD_POLICY;
  const captchaConfig = captchaData?.captchaConfig ?? { enabled: false, siteKey: '' };
  // The captcha provider drives which widget we render. In dev mode we
  // render a passive stub that auto-fills the bypass token so tests and
  // local docker-compose flows work without a Cloudflare/hCaptcha account.
  //
  // Both values are read at runtime (window.__RUNTIME_CONFIG__) so the
  // production image can be promoted across environments without rebuild.
  // See web/src/lib/runtime-config.ts and audit H-08.
  const captchaProvider = getRuntimeConfig('CAPTCHA_PROVIDER') || 'dev';
  const devBypassToken = getRuntimeConfig('DEV_CAPTCHA_BYPASS_TOKEN') || 'dev-ok';
  const regMode = regModeData?.registrationMode?.mode ?? 'closed';
  const inviteRequired = regModeData?.registrationMode?.inviteCodeRequired ?? false;
  const registrationOpen = regMode === 'open' || regMode === 'invite';
  const siteName = siteData?.siteConfig?.siteName || t('auth.platform_name');
  const siteSubtitle = siteData?.siteConfig?.subtitle || t('auth.platform_slogan');
  const siteLogoUrl = siteData?.siteConfig?.logoUrl || '';
  const siteInitial = siteName.trim().charAt(0).toUpperCase() || 'R';
  const redirectState = location.state as LoginLocationState | null;
  const from = redirectState?.from;
  const postLoginPath = from?.pathname && from.pathname !== '/login'
    ? `${from.pathname}${from.search ?? ''}${from.hash ?? ''}`
    : '/dashboard';

  const getPostAuthRedirect = useCallback((user?: AuthRedirectUser | null) => {
    const requiresPasswordChange = user?.require_password_change ?? user?.requirePasswordChange;
    return requiresPasswordChange ? '/change-password' : postLoginPath;
  }, [postLoginPath]);

  useEffect(() => {
    if (isAuthenticated) {
      navigate(getPostAuthRedirect(authUser), { replace: true });
    }
  }, [authUser, getPostAuthRedirect, isAuthenticated, navigate]);

  // When the dev backend is active, prefill the bypass token so the
  // signup form doesn't block on a captcha widget that doesn't exist.
  // The server still verifies it — the dev backend just accepts the
  // configured literal.
  useEffect(() => {
    if (captchaProvider === 'dev' || captchaProvider === 'disabled') {
      setCaptchaToken(devBypassToken);
    }
  }, [captchaProvider, devBypassToken]);

  // Fetch available OAuth2 providers
  const [oauthProviders, setOauthProviders] = useState<Array<{ id: string; name: string }>>([]);
  const [oauthProviderError, setOauthProviderError] = useState<string | null>(null);
  useEffect(() => {
    let cancelled = false;
    fetch('/auth/oauth2/providers', { headers: { Accept: 'application/json' } })
      .then(async (r) => {
        const contentType = r.headers.get('content-type') || '';
        if (!r.ok || !contentType.includes('application/json')) {
          throw new Error('OAuth provider endpoint unavailable');
        }
        return r.json();
      })
      .then(data => {
        if (cancelled) return;
        setOauthProviders(data.providers || []);
        setOauthProviderError(null);
      })
      .catch(() => {
        if (cancelled) return;
        setOauthProviders([]);
        setOauthProviderError(t('auth.oauth_unavailable'));
      });
    return () => {
      cancelled = true;
    };
  }, [t]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    // H-04: reset stale inline errors on every submit. Without this a user
    // who fixes the password but not the email still sees the green-checked
    // password field show "must contain a digit" until they touch it.
    setFieldErrors({});

    // CAPTCHA validation — inline next to the widget, not a toast.
    if (captchaConfig.enabled && !captchaToken) {
      setFieldErrors({ captchaToken: t('auth.captcha_required') });
      return;
    }

    // Registration-time client checks. Server is still the authority but
    // catching these before the round-trip gives the user inline feedback.
    if (!isLogin) {
      const localFieldErrs: Record<string, string> = {};
      const pwResult = validatePassword(formData.password, passwordPolicy);
      if (!pwResult.meetsAllRules) {
        localFieldErrs.password = pwResult.message || describePolicy(passwordPolicy);
      }
      if (formData.password !== formData.confirmPassword) {
        localFieldErrs.confirmPassword = t('auth.passwords_do_not_match');
      }
      if (Object.keys(localFieldErrs).length > 0) {
        setFieldErrors(localFieldErrs);
        return;
      }
    }

    setLoading(true);

    try {
      let authenticatedUser: AuthRedirectUser | null = null;

      if (isLogin) {
        const result = await loginMut({
          variables: { input: { email: formData.email, password: formData.password, captchaToken } },
        });
        if (result.error) throw apolloErrorToAuthError(result.error);
        const resp = (result.data as any)?.login;
        if (!resp?.token) throw new AuthError(t('auth.invalid_credentials'));
        authenticatedUser = resp.user;
        setAuth(resp.token, resp.user);
        toast.success(t('auth.welcome_back'));
      } else {
        const registerInput: Record<string, string | null> = {
          email: formData.email,
          password: formData.password,
          name: formData.name,
          captchaToken: captchaToken ?? '',
        };
        if (inviteRequired && formData.inviteCode) {
          registerInput.inviteCode = formData.inviteCode;
        }
        const result = await registerMut({
          variables: { input: registerInput },
        });
        if (result.error) throw apolloErrorToAuthError(result.error);
        const resp = (result.data as any)?.register;
        if (!resp?.token) throw new AuthError(t('auth.registration_failed'));
        authenticatedUser = resp.user;
        setAuth(resp.token, resp.user);
        // C-01: the user is logged in but not yet verified. The dashboard
        // shows an inline banner with a resend button; we surface the
        // success here as a slightly longer toast to set expectations.
        toast.success(t('auth.account_created_check_email'), { duration: 6000 });
      }
      navigate(getPostAuthRedirect(authenticatedUser), { replace: true });
    } catch (err: unknown) {
      const code = err instanceof AuthError ? err.code : undefined;
      const msg = err instanceof Error ? err.message : '';
      const allErrors = err instanceof AuthError ? err.errors : [];

      // H-04: collect every VALIDATION error (the server may flag multiple
      // fields in one round-trip) into the field-keyed map. Fields without
      // extensions.field fall back to a generic toast so the user isn't
      // left guessing. Non-VALIDATION errors fall through to the existing
      // toast logic below.
      const validationErrors = allErrors.filter((e) => e.code === 'VALIDATION');
      if (validationErrors.length > 0) {
        const nextFieldErrs: Record<string, string> = {};
        const unmapped: string[] = [];
        for (const v of validationErrors) {
          if (v.field) {
            nextFieldErrs[v.field] = v.message;
          } else {
            unmapped.push(v.message);
          }
        }
        if (Object.keys(nextFieldErrs).length > 0) {
          setFieldErrors(nextFieldErrs);
        }
        // Surface anything we couldn't pin to a field — keeps the toast
        // useful even when the server returns a top-level validation.
        if (unmapped.length > 0) {
          toast.error(unmapped.join('\n'));
        } else if (Object.keys(nextFieldErrs).length === 0) {
          // Defensive: VALIDATION without field and without message — show
          // generic guidance rather than nothing at all.
          toast.error(t('auth.validation_fix_below'));
        }
      } else if (isLogin) {
        // Branch on the stable server-issued code first; fall back to message
        // text only when the server hasn't been retrofitted with extensions.code.
        if (code === 'INVALID_CREDENTIALS') {
          // Backend collapses "account not found" / "password incorrect" /
          // generic "invalid credentials" into the same code as a defense
          // against email enumeration. Show a single neutral message.
          toast.error(t('auth.invalid_credentials'));
        } else if (code === 'RATE_LIMITED') {
          const match = msg.match(/in (\d+) minutes/);
          if (match) toast.error(t('auth.rate_limit_login', { minutes: match[1] }));
          else toast.error(t('auth.rate_limit_login_later'));
        } else if (code === 'ACCOUNT_DISABLED') {
          toast.error(t('auth.account_disabled') || msg);
        } else {
          toast.error(msg || t('auth.invalid_credentials'));
        }
      } else {
        toast.error(msg || t('auth.registration_failed'));
      }
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
      const result = await discoverSsoMut({ variables: { email: formData.email } });
      const redirectUrl = (result.data as any)?.discoverSso?.redirectUrl;
      if (!redirectUrl) throw new Error(t('auth.sso_failed'));
      window.location.href = redirectUrl;
    } catch (err: any) {
      toast.error(err.message || t('auth.sso_failed'));
      setLoading(false);
    }
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setFormData((prev) => ({
      ...prev,
      [name]: value,
    }));
    // H-04: clear the inline error for the field the user is actively
    // editing. Without this, after a failed submit the error remains
    // visible while the user types a fix, which feels broken.
    if (fieldErrors[name]) {
      setFieldErrors((prev) => {
        const next = { ...prev };
        delete next[name];
        return next;
      });
    }
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
          <div className="w-16 h-16 bg-apple-blue rounded-2xl flex items-center justify-center shadow-lg mx-auto mb-5 overflow-hidden">
            {siteLogoUrl ? (
              <img src={siteLogoUrl} alt="" className="h-full w-full object-contain p-2" />
            ) : (
              <span className="text-white font-bold text-2xl">{siteInitial}</span>
            )}
          </div>
          <h1 className="text-3xl font-semibold text-apple-gray-900 mb-2">{siteName}</h1>
          <p className="text-apple-gray-500">{siteSubtitle}</p>
        </div>

        <div className="card">
          {/* Segmented Control — role=tablist so screen readers announce the
              two buttons as a coordinated pair and report which is active. */}
          <div role="tablist" aria-label={t('auth.login_register_toggle')} className="flex bg-apple-gray-100 rounded-xl p-1 mb-8 border border-apple-gray-200">
            <button
              role="tab"
              aria-selected={isLogin}
              type="button"
              onClick={() => setIsLogin(true)}
              className={`flex-1 py-2 text-sm font-semibold rounded-lg transition-all duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue ${
                isLogin
                  ? 'bg-white text-apple-blue shadow-sm border border-apple-gray-200'
                  : 'text-apple-gray-500 hover:text-apple-gray-700'
              }`}
            >
              {t('auth.login')}
            </button>
            <button
              role="tab"
              aria-selected={!isLogin}
              type="button"
              onClick={() => setIsLogin(false)}
              disabled={!registrationOpen}
              className={`flex-1 py-2 text-sm font-semibold rounded-lg transition-all duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue ${
                !isLogin
                  ? 'bg-white text-apple-blue shadow-sm border border-apple-gray-200'
                  : !registrationOpen
                    ? 'text-apple-gray-400 cursor-not-allowed'
                    : 'text-apple-gray-500 hover:text-apple-gray-700'
              }`}
              title={!registrationOpen ? t('auth.registration_closed_hint') : undefined}
            >
              {t('auth.register')}
            </button>
          </div>

          {/* Registration closed hint */}
          {isLogin && !registrationOpen && (
            <div className="mb-6 p-3 bg-amber-50 border border-amber-200 rounded-xl text-sm text-amber-700">
              {t('auth.registration_closed_inline')}
            </div>
          )}
          {!isLogin && !registrationOpen && (
            <div className="mb-6 p-3 bg-amber-50 border border-amber-200 rounded-xl text-sm text-amber-700">
              {t('auth.registration_closed')}
            </div>
          )}

          {/* Social / OAuth Login (Prominent Top Position) */}
          {oauthProviders.length > 0 && !isSsoMode && (
            <div className="mb-6">
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
              <div className="relative mt-6 mb-2">
                <div className="absolute inset-0 flex items-center">
                  <div className="w-full border-t border-apple-gray-200" />
                </div>
                <div className="relative flex justify-center text-xs">
                  <span className="bg-white px-3 text-apple-gray-400 font-medium">
                    {t('auth.or_continue_with')}
                  </span>
                </div>
              </div>
            </div>
          )}
          {oauthProviderError && !isSsoMode && (
            <div role="status" className="mb-6 rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-700">
              {oauthProviderError}
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
                  aria-invalid={!!fieldErrors.name}
                  aria-describedby={fieldErrors.name ? 'name-error' : undefined}
                />
                {fieldErrors.name && (
                  <p id="name-error" role="alert" className="text-xs mt-1.5 text-red-600">{fieldErrors.name}</p>
                )}
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
                aria-invalid={!!fieldErrors.email}
                aria-describedby={fieldErrors.email ? 'email-error' : undefined}
              />
              {fieldErrors.email && (
                <p id="email-error" role="alert" className="text-xs mt-1.5 text-red-600">{fieldErrors.email}</p>
              )}
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
                minLength={passwordPolicy.minLength}
                aria-invalid={!!fieldErrors.password}
                aria-describedby={fieldErrors.password ? 'password-error' : 'password-hint'}
              />

              {/* Inline hint / live validation. Order of precedence:
                  1. Server-issued VALIDATION error (red, role=alert) wins
                     because it carries the authoritative rejection reason.
                  2. Live client validation during typing (amber) gives a
                     hint before submit.
                  3. Empty field → policy description (gray) — driven by the
                     PASSWORD_POLICY query so the copy never falls out of
                     sync with the server. H-04. */}
              {fieldErrors.password ? (
                <p id="password-error" role="alert" className="text-xs mt-1.5 text-red-600">{fieldErrors.password}</p>
              ) : !isLogin ? (
                (() => {
                  const result = validatePassword(formData.password, passwordPolicy);
                  if (formData.password.length === 0) {
                    return (
                      <p id="password-hint" className="text-xs mt-1.5 text-apple-gray-500">{describePolicy(passwordPolicy)}</p>
                    );
                  }
                  if (result.meetsAllRules) {
                    return <p id="password-hint" className="text-xs mt-1.5 text-green-600">{t('auth.password_looks_good')}</p>;
                  }
                  return <p id="password-hint" className="text-xs mt-1.5 text-amber-600">{result.message}</p>;
                })()
              ) : null}

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

              {/* Confirm password — registration only. Client-side match check;
                  the server itself doesn't see this value (we don't send it). */}
              {!isLogin && (
                <div>
                  <label htmlFor="confirmPassword" className="label">
                    {t('auth.confirm_password')}
                  </label>
                  <input
                    type="password"
                    id="confirmPassword"
                    name="confirmPassword"
                    value={formData.confirmPassword}
                    onChange={handleInputChange}
                    className="input"
                    placeholder={t('auth.confirm_password_placeholder')}
                    required
                    autoComplete="new-password"
                    minLength={passwordPolicy.minLength}
                    aria-invalid={!!fieldErrors.confirmPassword || (formData.confirmPassword.length > 0 && formData.confirmPassword !== formData.password)}
                    aria-describedby={fieldErrors.confirmPassword ? 'confirm-password-error' : undefined}
                  />
                  {/* Server VALIDATION wins over the live mismatch hint —
                      e.g. a 422 with field=confirmPassword should not be
                      hidden by a stale "Passwords do not match." */}
                  {fieldErrors.confirmPassword ? (
                    <p id="confirm-password-error" role="alert" className="text-xs mt-1.5 text-red-600">{fieldErrors.confirmPassword}</p>
                  ) : (
                    formData.confirmPassword.length > 0 && formData.confirmPassword !== formData.password && (
                      <p className="text-xs mt-1.5 text-amber-600">{t('auth.passwords_do_not_match')}</p>
                    )
                  )}
                </div>
              )}
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
                  aria-invalid={!!fieldErrors.inviteCode}
                  aria-describedby={fieldErrors.inviteCode ? 'invite-code-error' : undefined}
                />
                {fieldErrors.inviteCode && (
                  <p id="invite-code-error" role="alert" className="text-xs mt-1.5 text-red-600">{fieldErrors.inviteCode}</p>
                )}
              </div>
            )}

            {/* Captcha widget. The provider picked at build time decides what
                we render: turnstile / hcaptcha get the real widget; dev gets
                a passive stub showing the bypass token; disabled renders
                nothing. The server is the source of truth either way. */}
            {!isSsoMode && captchaConfig.enabled && captchaProvider === 'turnstile' && captchaConfig.siteKey && (
              <div className="flex flex-col items-center gap-1.5">
                <Turnstile
                  ref={turnstileRef}
                  siteKey={captchaConfig.siteKey}
                  onSuccess={(token) => setCaptchaToken(token)}
                  onExpire={() => setCaptchaToken(null)}
                  onError={() => setCaptchaToken(null)}
                  options={{ theme: 'light', size: 'normal' }}
                />
                {fieldErrors.captchaToken && (
                  <p role="alert" className="text-xs text-red-600">{fieldErrors.captchaToken}</p>
                )}
              </div>
            )}
            {!isSsoMode && captchaConfig.enabled && captchaProvider === 'dev' && (
              <div>
                <div className="rounded-xl border border-apple-gray-200 bg-apple-gray-50 px-3 py-2 text-xs text-apple-gray-500">
                  Captcha (dev mode — token = <code>{devBypassToken}</code>)
                </div>
                {fieldErrors.captchaToken && (
                  <p role="alert" className="text-xs mt-1.5 text-red-600">{fieldErrors.captchaToken}</p>
                )}
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
          {isLogin && !isSsoMode && (
            <div className="mt-6 text-center">
              <button
                type="button"
                onClick={() => setIsSsoMode(true)}
                className="text-sm font-medium text-apple-gray-500 hover:text-apple-gray-800 transition-colors"
              >
                {t('auth.sso_login')}
              </button>
            </div>
          )}
          {isLogin && isSsoMode && (
            <div className="mt-6 text-center">
              <button
                type="button"
                onClick={() => setIsSsoMode(false)}
                className="text-sm font-medium text-apple-gray-500 hover:text-apple-gray-800 transition-colors"
              >
                {t('auth.sso_back')}
              </button>
            </div>
          )}
        </div>
      </motion.div>
    </div>
  );
}

export default LoginPage;

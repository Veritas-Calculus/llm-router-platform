/* eslint-disable @typescript-eslint/no-explicit-any */
import { useEffect, useRef, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useMutation } from '@apollo/client/react';
import { gql } from '@apollo/client';
import { useAuthStore } from '@/stores/authStore';

const EXCHANGE_OAUTH_CODE = gql`
  mutation ExchangeOAuthCode {
    exchangeOAuthCode {
      token
      refreshToken
      user { id email name role isActive mfaEnabled emailVerified }
    }
  }
`;

/**
 * OAuthCallbackPage handles the redirect from the backend OAuth2/SSO callback.
 *
 * The backend no longer ships the JWT in the URL (that path leaked tokens to
 * browser history, Referer headers, and CDN/proxy access logs). Instead it
 * sets a short-lived HttpOnly + Secure exchange cookie and redirects here.
 * We trade that cookie for a real AuthPayload via a GraphQL mutation.
 */
export default function OAuthCallbackPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const setAuth = useAuthStore((s) => s.setAuth);
  const [error, setError] = useState('');
  const [exchange] = useMutation<any>(EXCHANGE_OAUTH_CODE);
  // useEffect runs twice under React.StrictMode in dev — make sure the
  // single-use cookie isn't redeemed twice (the second call would 401).
  const didRunRef = useRef(false);

  useEffect(() => {
    if (didRunRef.current) return;
    didRunRef.current = true;

    const errorMsg = searchParams.get('error');
    if (errorMsg) {
      setError(errorMsg);
      return;
    }

    (async () => {
      try {
        const result = await exchange();
        if (result.error) throw new Error(result.error.message);
        const payload = (result.data as any)?.exchangeOAuthCode;
        if (!payload?.token || !payload?.user) {
          throw new Error('Authentication response was incomplete');
        }
        setAuth(payload.token, payload.user, payload.refreshToken ?? null);
        navigate('/dashboard', { replace: true });
      } catch (err: any) {
        setError(err.message || 'Authentication failed');
      }
    })();
  }, [searchParams, exchange, setAuth, navigate]);

  if (error) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-apple-gray-50">
        <div className="text-center max-w-md">
          <div className="w-16 h-16 bg-red-100 rounded-full flex items-center justify-center mx-auto mb-4">
            <span className="text-2xl font-bold text-red-500">X</span>
          </div>
          <h1 className="text-xl font-semibold text-apple-gray-900 mb-2">Authentication Failed</h1>
          <p className="text-sm text-apple-gray-500 mb-6">{error}</p>
          <button onClick={() => navigate('/login')} className="btn-primary px-6 py-2 rounded-xl text-sm font-semibold">
            Back to Login
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-apple-gray-50">
      <div className="text-center">
        <div className="w-10 h-10 border-3 border-apple-blue border-t-transparent rounded-full animate-spin mx-auto mb-4" />
        <p className="text-sm text-apple-gray-500">Completing sign in...</p>
      </div>
    </div>
  );
}

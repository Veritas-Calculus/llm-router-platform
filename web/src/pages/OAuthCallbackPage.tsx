import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAuthStore } from '@/stores/authStore';

/**
 * OAuthCallbackPage handles the redirect from the backend OAuth2 callback.
 * It extracts the JWT token from the URL, stores it, and redirects to dashboard.
 */
export default function OAuthCallbackPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const setAuth = useAuthStore((s) => s.setAuth);
  const [error, setError] = useState('');

  useEffect(() => {
    const token = searchParams.get('token');
    const errorMsg = searchParams.get('error');

    if (errorMsg) {
      setError(errorMsg);
      return;
    }

    if (!token) {
      setError('No authentication token received');
      return;
    }

    // Decode JWT to get user info (without verification — the backend verified it)
    try {
      const payload = JSON.parse(atob(token.split('.')[1]));
      const user = {
        id: payload.sub,
        email: payload.email,
        role: payload.role,
        name: payload.email.split('@')[0],
        isActive: true,
      };
      setAuth(token, user);
      navigate('/dashboard', { replace: true });
    } catch {
      setError('Invalid authentication token');
    }
  }, [searchParams, setAuth, navigate]);

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

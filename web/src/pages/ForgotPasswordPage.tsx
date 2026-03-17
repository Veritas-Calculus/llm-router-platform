import { useState } from 'react';
import { Link } from 'react-router-dom';
import { motion } from 'framer-motion';
import toast from 'react-hot-toast';
import { authApi, getApiErrorMessage } from '@/lib/api';

function ForgotPasswordPage() {
  const [email, setEmail] = useState('');
  const [loading, setLoading] = useState(false);
  const [submitted, setSubmitted] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);

    try {
      await authApi.forgotPassword({ email });
      setSubmitted(true);
      toast.success('Reset link sent if account exists');
    } catch (error) {
      toast.error(getApiErrorMessage(error, 'Failed to process request'));
    } finally {
      setLoading(false);
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
        <div className="text-center mb-8">
          <h1 className="text-3xl font-semibold text-apple-gray-900 mb-2">Reset Password</h1>
          <p className="text-apple-gray-500">We'll send a reset link to your email</p>
        </div>

        <div className="card">
          {!submitted ? (
            <form onSubmit={handleSubmit} className="space-y-4">
              <div>
                <label htmlFor="email" className="label">
                  Email Address
                </label>
                <input
                  type="email"
                  id="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="input"
                  placeholder="Enter your registered email"
                  required
                />
              </div>

              <button type="submit" className="btn-primary w-full py-3" disabled={loading}>
                {loading ? 'Sending...' : 'Send Reset Link'}
              </button>

              <div className="text-center mt-4">
                <Link to="/login" className="text-apple-blue hover:underline text-sm font-medium">
                  Back to Login
                </Link>
              </div>
            </form>
          ) : (
            <div className="text-center py-4">
              <div className="mb-4 text-apple-green">
                <svg
                  className="w-16 h-16 mx-auto"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
              </div>
              <h2 className="text-xl font-medium text-apple-gray-900 mb-2">Check Your Email</h2>
              <p className="text-apple-gray-500 mb-6">
                If an account exists for {email}, you will receive a password reset link shortly.
              </p>
              <Link to="/login" className="btn-secondary inline-block px-6 py-2">
                Return to Login
              </Link>
            </div>
          )}
        </div>
      </motion.div>
    </div>
  );
}

export default ForgotPasswordPage;

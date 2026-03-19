import { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { motion } from 'framer-motion';
import toast from 'react-hot-toast';
import { useMutation } from '@apollo/client/react';
import { LOGIN, REGISTER } from '@/lib/graphql/operations';
import { useAuthStore } from '@/stores/authStore';

function LoginPage() {
  const navigate = useNavigate();
  const setAuth = useAuthStore((state) => state.setAuth);
  const [loginMut] = useMutation(LOGIN);
  const [registerMut] = useMutation(REGISTER);
  const [isLogin, setIsLogin] = useState(true);
  const [loading, setLoading] = useState(false);
  const [formData, setFormData] = useState({
    email: '',
    password: '',
    name: '',
  });

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);

    try {
      if (isLogin) {
        const { data } = await loginMut({
          variables: { input: { email: formData.email, password: formData.password } },
        });
        const resp = (data as any)?.login;
        setAuth(resp.token, resp.user);
        toast.success('Welcome back!');
      } else {
        const { data } = await registerMut({
          variables: { input: { email: formData.email, password: formData.password, name: formData.name } },
        });
        const resp = (data as any)?.register;
        setAuth(resp.token, resp.user);
        toast.success('Account created successfully!');
      }
      // Read from store (just set above) to determine where to navigate
      const user = useAuthStore.getState().user;
      navigate(user?.require_password_change ? '/change-password' : '/dashboard');
    } catch {
      toast.error(isLogin ? 'Invalid credentials' : 'Registration failed');
    } finally {
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
          <h1 className="text-3xl font-semibold text-apple-gray-900 mb-2">LLM Router Platform</h1>
          <p className="text-apple-gray-500">Intelligent routing for LLM APIs</p>
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
              Sign In
            </button>
            <button
              onClick={() => setIsLogin(false)}
              className={`flex-1 py-2 text-sm font-semibold rounded-lg transition-all duration-200 ${
                !isLogin
                  ? 'bg-white text-apple-blue shadow-sm border border-apple-gray-200'
                  : 'text-apple-gray-500 hover:text-apple-gray-700'
              }`}
            >
              Sign Up
            </button>
          </div>

          <form onSubmit={handleSubmit} className="space-y-5">
            {!isLogin && (
              <div>
                <label htmlFor="name" className="label">
                  Name
                </label>
                <input
                  type="text"
                  id="name"
                  name="name"
                  value={formData.name}
                  onChange={handleInputChange}
                  className="input"
                  placeholder="Enter your name"
                  required={!isLogin}
                />
              </div>
            )}

            <div>
              <label htmlFor="email" className="label">
                Email
              </label>
              <input
                type="email"
                id="email"
                name="email"
                value={formData.email}
                onChange={handleInputChange}
                className="input"
                placeholder="Enter your email"
                required
              />
            </div>

            <div>
              <label htmlFor="password" className="label">
                Password
              </label>
              <input
                type="password"
                id="password"
                name="password"
                value={formData.password}
                onChange={handleInputChange}
                className="input"
                placeholder="Enter your password"
                required
                minLength={6}
              />
              {isLogin && (
                <div className="flex justify-end mt-1.5">
                  <Link
                    to="/forgot-password"
                    className="text-sm text-apple-blue hover:underline font-medium"
                  >
                    Forgot Password?
                  </Link>
                </div>
              )}
            </div>

            <button type="submit" className="btn-primary w-full py-3 rounded-xl text-base font-semibold mt-2" disabled={loading}>
              {loading ? (
                <span className="flex items-center justify-center gap-2">
                  <svg
                    className="animate-spin h-5 w-5"
                    xmlns="http://www.w3.org/2000/svg"
                    fill="none"
                    viewBox="0 0 24 24"
                  >
                    <circle
                      className="opacity-25"
                      cx="12"
                      cy="12"
                      r="10"
                      stroke="currentColor"
                      strokeWidth="4"
                    />
                    <path
                      className="opacity-75"
                      fill="currentColor"
                      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                    />
                  </svg>
                  Processing...
                </span>
              ) : isLogin ? (
                'Sign In'
              ) : (
                'Create Account'
              )}
            </button>
          </form>
        </div>
      </motion.div>
    </div>
  );
}

export default LoginPage;

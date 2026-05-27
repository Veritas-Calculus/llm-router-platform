/* eslint-disable @typescript-eslint/no-explicit-any */
import { useEffect, useState } from 'react';
import { useSearchParams, Link } from 'react-router-dom';
import { motion } from 'framer-motion';
import { useMutation } from '@apollo/client/react';
import { VERIFY_EMAIL } from '@/lib/graphql/operations';
import { useTranslation } from '@/lib/i18n';

type Status = 'verifying' | 'success' | 'error';

function VerifyEmailPage() {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const token = searchParams.get('token');
  const [status, setStatus] = useState<Status>(token ? 'verifying' : 'error');
  const [errorMsg, setErrorMsg] = useState('');
  const [verifyEmail] = useMutation(VERIFY_EMAIL);

  useEffect(() => {
    if (!token) {
      setErrorMsg(t('auth.verify_missing_token'));
      return;
    }

    verifyEmail({ variables: { token } })
      .then(() => setStatus('success'))
      .catch((err: any) => {
        setStatus('error');
        setErrorMsg(err?.message || t('auth.verify_failed'));
      });
  }, [token, verifyEmail, t]);

  return (
    <div className="min-h-screen flex items-center justify-center bg-apple-gray-50 px-4">
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4 }}
        className="w-full max-w-md"
      >
        <div className="text-center mb-10">
          <div className="w-16 h-16 bg-gradient-to-br from-apple-blue to-blue-600 rounded-2xl flex items-center justify-center shadow-lg mx-auto mb-5">
            <span className="text-white font-bold text-2xl">R</span>
          </div>
          <h1 className="text-3xl font-semibold text-apple-gray-900 mb-2">{t('auth.verify_title')}</h1>
        </div>

        <div className="card text-center">
          {status === 'verifying' && (
            <div className="py-8 flex flex-col items-center gap-4">
              <div className="w-10 h-10 border-[3px] border-apple-gray-200 border-t-apple-blue rounded-full animate-spin" />
              <p className="text-apple-gray-500 text-base">{t('auth.verify_checking')}</p>
            </div>
          )}

          {status === 'success' && (
            <div className="py-8 flex flex-col items-center gap-4">
              <div className="w-16 h-16 bg-green-50 rounded-full flex items-center justify-center">
                <svg className="w-8 h-8 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                </svg>
              </div>
              <h2 className="text-xl font-semibold text-apple-gray-900">{t('auth.verify_success_heading')}</h2>
              <p className="text-apple-gray-500 text-sm max-w-xs">
                {t('auth.verify_success_body')}
              </p>
              <Link
                to="/dashboard"
                className="btn-primary px-6 py-2.5 rounded-xl text-sm font-semibold mt-2 inline-block"
              >
                {t('auth.verify_go_dashboard')}
              </Link>
            </div>
          )}

          {status === 'error' && (
            <div className="py-8 flex flex-col items-center gap-4">
              <div className="w-16 h-16 bg-red-50 rounded-full flex items-center justify-center">
                <svg className="w-8 h-8 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </div>
              <h2 className="text-xl font-semibold text-apple-gray-900">{t('auth.verify_failed_heading')}</h2>
              <p className="text-apple-gray-500 text-sm max-w-xs">
                {errorMsg || t('auth.verify_failed_body')}
              </p>
              <Link
                to="/login"
                className="btn-primary px-6 py-2.5 rounded-xl text-sm font-semibold mt-2 inline-block"
              >
                {t('auth.verify_back_to_login')}
              </Link>
            </div>
          )}
        </div>
      </motion.div>
    </div>
  );
}

export default VerifyEmailPage;

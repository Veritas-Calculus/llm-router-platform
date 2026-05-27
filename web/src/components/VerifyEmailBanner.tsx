import { useState } from 'react';
import toast from 'react-hot-toast';
import { useMutation, useQuery } from '@apollo/client/react';
import { EnvelopeIcon } from '@heroicons/react/24/outline';
import { ME, RESEND_VERIFICATION_EMAIL } from '@/lib/graphql/operations';
import { useAuthStore } from '@/stores/authStore';
import { useTranslation } from '@/lib/i18n';

/**
 * VerifyEmailBanner is rendered above every authenticated route. It reads
 * the verification status from the Apollo cache (refreshed by the
 * bootstrap `me` query) and prompts the user to confirm their email if
 * they haven't yet. C-01: the $5 welcome credit is gated on verification,
 * so we surface the unlock prominently.
 *
 * The component renders nothing when the user is already verified or when
 * we haven't yet loaded the user record — the unauthenticated case is
 * handled by the parent layout's gating.
 */
export default function VerifyEmailBanner() {
  const { t } = useTranslation();
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const { data } = useQuery(ME, {
    skip: !isAuthenticated,
    fetchPolicy: 'cache-first',
  });
  const [resendMut] = useMutation(RESEND_VERIFICATION_EMAIL);
  const [busy, setBusy] = useState(false);

  if (!isAuthenticated || !data?.me) return null;
  if (data.me.emailVerified) return null;

  const onResend = async () => {
    if (busy) return;
    setBusy(true);
    try {
      const res = await resendMut();
      if (res.error) throw res.error;
      toast.success(t('auth.verify_email_resend_success'));
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to resend';
      toast.error(msg);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div
      role="status"
      className="mx-4 mt-4 sm:mx-6 lg:mx-8 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 flex items-start gap-3"
    >
      <EnvelopeIcon className="w-5 h-5 mt-0.5 text-amber-600 shrink-0" aria-hidden="true" />
      <div className="flex-1 min-w-0">
        <p className="text-sm font-semibold text-amber-900">{t('auth.verify_email_banner_title')}</p>
        <p className="text-xs text-amber-800 mt-0.5">{t('auth.verify_email_banner_body')}</p>
      </div>
      <button
        type="button"
        onClick={onResend}
        disabled={busy}
        className="text-xs font-semibold text-amber-900 hover:text-amber-700 underline disabled:opacity-50 disabled:no-underline whitespace-nowrap"
      >
        {busy ? '…' : t('auth.verify_email_resend')}
      </button>
    </div>
  );
}

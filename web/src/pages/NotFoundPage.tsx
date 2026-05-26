import { Link, useLocation } from 'react-router-dom';
import { ArrowLeftIcon, MagnifyingGlassIcon } from '@heroicons/react/24/outline';
import { useTranslation } from '@/lib/i18n';

export default function NotFoundPage() {
  const { t } = useTranslation();
  const location = useLocation();
  const dashboardPath = location.pathname.startsWith('/admin') ? '/admin/dashboard' : '/dashboard';

  return (
    <div className="min-h-[55vh] flex items-center justify-center px-4">
      <div className="w-full max-w-lg text-center">
        <div className="mx-auto mb-6 flex h-14 w-14 items-center justify-center rounded-2xl bg-apple-gray-100 text-apple-gray-500">
          <MagnifyingGlassIcon className="h-7 w-7" aria-hidden="true" />
        </div>
        <h1 className="text-2xl font-semibold text-apple-gray-900">
          {t('not_found.title')}
        </h1>
        <p className="mt-3 text-sm leading-6 text-apple-gray-500">
          {t('not_found.description')}
        </p>
        <p className="mt-3 rounded-xl bg-apple-gray-100 px-3 py-2 font-mono text-xs text-apple-gray-500">
          {location.pathname}
        </p>
        <div className="mt-6 flex justify-center">
          <Link to={dashboardPath} className="btn btn-primary inline-flex items-center gap-2">
            <ArrowLeftIcon className="h-4 w-4" aria-hidden="true" />
            {t('not_found.back_to_dashboard')}
          </Link>
        </div>
      </div>
    </div>
  );
}

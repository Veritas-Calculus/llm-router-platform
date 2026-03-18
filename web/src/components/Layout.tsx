import { useState, useEffect } from 'react';
import { Outlet, NavLink, useNavigate, useLocation } from 'react-router-dom';
import { motion, AnimatePresence } from 'framer-motion';
import clsx from 'clsx';
import {
  HomeIcon,
  ChartBarIcon,
  KeyIcon,
  CommandLineIcon,
  HeartIcon,
  CreditCardIcon,
  SparklesIcon,
  ServerStackIcon,
  GlobeAltIcon,
  Cog6ToothIcon,
  ArrowRightOnRectangleIcon,
  DocumentTextIcon,
  Bars3Icon,
  UsersIcon,
  UserIcon,
} from '@heroicons/react/24/outline';
import { useAuthStore } from '@/stores/authStore';
import { useTranslation } from 'react-i18next';
import NotificationCenter from '@/components/NotificationCenter';

const navigation = [
  { key: 'nav.dashboard', href: '/dashboard', icon: HomeIcon },
  { key: 'nav.usage', href: '/usage', icon: ChartBarIcon },
  { key: 'nav.api_keys', href: '/api-keys', icon: KeyIcon },
  { key: 'nav.plans', href: '/plans', icon: SparklesIcon },
  { key: 'nav.billing', href: '/billing', icon: CreditCardIcon },
  { key: 'nav.profile', href: '/profile', icon: UserIcon },
  { key: 'nav.docs', href: '/docs', icon: DocumentTextIcon },
];

const adminNavItems = [
  { key: 'nav.dashboard', href: '/dashboard', icon: HomeIcon },
  { key: 'nav.usage', href: '/usage', icon: ChartBarIcon },
  { key: 'nav.api_keys', href: '/api-keys', icon: KeyIcon },
  { key: 'nav.plans', href: '/plans', icon: SparklesIcon },
  { key: 'nav.billing', href: '/billing', icon: CreditCardIcon },
  { key: 'nav.profile', href: '/profile', icon: UserIcon },
  // Admin-only sections
  { key: 'nav.users', href: '/users', icon: UsersIcon },
  { key: 'nav.health', href: '/health', icon: HeartIcon },
  { key: 'nav.providers', href: '/providers', icon: ServerStackIcon },
  { key: 'nav.mcp', href: '/mcp', icon: CommandLineIcon },
  { key: 'nav.proxies', href: '/proxies', icon: GlobeAltIcon },
  { key: 'nav.settings', href: '/settings', icon: Cog6ToothIcon },
  { key: 'nav.docs', href: '/docs', icon: DocumentTextIcon },
];

function Layout() {
  const { user, logout, isAdmin } = useAuthStore();
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);

  const navItems = isAdmin ? adminNavItems : navigation;

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  const toggleLanguage = () => {
    const newLang = i18n.language === 'en' ? 'zh-CN' : 'en';
    i18n.changeLanguage(newLang);
  };

  // Close sidebar on route change (mobile)
  useEffect(() => {
    setIsSidebarOpen(false);
  }, [location.pathname]);

  return (
    <div className="min-h-screen bg-apple-gray-50 flex">
      {/* Mobile Sidebar Overlay */}
      <AnimatePresence>
        {isSidebarOpen && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={() => setIsSidebarOpen(false)}
            className="fixed inset-0 bg-black/20 backdrop-blur-sm z-40 lg:hidden"
          />
        )}
      </AnimatePresence>

      {/* Sidebar */}
      <aside
        className={clsx(
          'fixed inset-y-0 left-0 z-50 w-72 bg-white/80 backdrop-blur-xl border-r border-apple-gray-200 transform transition-transform duration-300 ease-in-out lg:relative lg:translate-x-0',
          isSidebarOpen ? 'translate-x-0' : '-translate-x-full'
        )}
      >
        <div className="flex flex-col h-full">
          <div className="p-8 flex items-center gap-3">
            <div className="w-10 h-10 bg-apple-blue rounded-xl flex items-center justify-center shadow-apple-blue">
              <span className="text-white font-bold text-xl">R</span>
            </div>
            <span className="text-xl font-bold bg-clip-text text-transparent bg-gradient-to-r from-apple-gray-900 to-apple-gray-600">
              Router
            </span>
          </div>

          <nav className="flex-1 px-4 space-y-1 overflow-y-auto">
            {navItems.map((item) => (
              <NavLink
                key={item.key}
                to={item.href}
                className={({ isActive }) =>
                  clsx(
                    'group flex items-center px-4 py-3 text-sm font-medium rounded-xl transition-all duration-200',
                    isActive
                      ? 'bg-apple-blue/5 text-apple-blue shadow-sm'
                      : 'text-apple-gray-600 hover:bg-apple-gray-50 hover:text-apple-gray-900'
                  )
                }
              >
                {({ isActive }) => (
                  <>
                    <item.icon
                      className={clsx(
                        isActive ? 'text-apple-blue' : 'text-apple-gray-400 group-hover:text-apple-gray-500',
                        'mr-3 h-6 w-6 shrink-0 transition-colors'
                      )}
                      aria-hidden="true"
                    />
                    <span className="flex-1">{t(item.key)}</span>
                    {item.key === 'nav.plans' && user?.balance !== undefined && (
                      <span className="ml-2 px-2 py-0.5 rounded-full text-[10px] font-bold bg-blue-50 text-apple-blue border border-blue-100">
                        ${user.balance.toFixed(2)}
                      </span>
                    )}
                  </>
                )}
              </NavLink>
            ))}
          </nav>

          <div className="p-4 border-t border-apple-gray-100 space-y-4">
            <div className="flex items-center gap-3 px-4 py-2">
              <div className="w-8 h-8 bg-apple-gray-100 rounded-full flex items-center justify-center text-apple-gray-600 font-bold border border-apple-gray-200">
                {user?.name?.charAt(0).toUpperCase() || 'U'}
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-sm font-semibold text-apple-gray-900 truncate">
                  {user?.name || 'User'}
                </p>
                <p className="text-xs text-apple-gray-500 truncate">{user?.email}</p>
              </div>
            </div>

            <div className="flex gap-2">
              <button
                onClick={toggleLanguage}
                className="flex-1 px-3 py-2 text-xs font-medium text-apple-gray-600 bg-apple-gray-50 rounded-lg hover:bg-apple-gray-100 transition-colors border border-apple-gray-200"
              >
                {i18n.language === 'en' ? '中文' : 'EN'}
              </button>
              <button
                onClick={handleLogout}
                className="px-3 py-2 text-xs font-medium text-apple-red bg-red-50 rounded-lg hover:bg-red-100 transition-colors border border-red-100 flex items-center gap-2"
              >
                <ArrowRightOnRectangleIcon className="w-4 h-4" />
                {t('auth.logout')}
              </button>
            </div>
          </div>
        </div>
      </aside>

      {/* Main Content */}
      <main className="flex-1 flex flex-col min-w-0 overflow-hidden">
        {/* Top Header (Mobile Only) */}
        <header className="bg-white/80 backdrop-blur-md border-b border-apple-gray-200 h-16 flex items-center justify-between px-4 lg:hidden sticky top-0 z-30">
          <button
            onClick={() => setIsSidebarOpen(true)}
            className="p-2 -ml-2 text-apple-gray-600"
          >
            <Bars3Icon className="w-6 h-6" />
          </button>
          <span className="font-bold text-lg">Router</span>
          <div className="w-10" /> {/* Spacer */}
        </header>

        {/* Global Notifications */}
        <div className="sticky top-0 z-20 pointer-events-none">
          <div className="max-w-7xl mx-auto px-4 py-2 flex justify-end">
            <div className="pointer-events-auto">
              <NotificationCenter />
            </div>
          </div>
        </div>

        <motion.div
          key={location.pathname}
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3 }}
          className="p-4 sm:p-6 lg:p-8"
        >
          <Outlet />
        </motion.div>
      </main>
    </div>
  );
}

export default Layout;

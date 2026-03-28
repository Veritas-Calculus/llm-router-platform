import { useState, useEffect, useRef } from 'react';
import { Outlet, NavLink, useNavigate, useLocation } from 'react-router-dom';
import { motion, AnimatePresence } from 'framer-motion';
import clsx from 'clsx';
import { useQuery } from '@apollo/client/react';
import { SITE_CONFIG_QUERY } from '@/lib/graphql/operations';
import {
  KeyIcon,
  ChartBarIcon,
  CreditCardIcon,
  DocumentTextIcon,
  UserIcon,
  HomeIcon,
  UsersIcon,
  MegaphoneIcon,
  SparklesIcon,
  TicketIcon,
  GlobeAltIcon,
  CommandLineIcon,
  DocumentDuplicateIcon,
  Cog6ToothIcon,
  ArrowRightOnRectangleIcon,
  Bars3Icon,
  ShieldCheckIcon,
  LanguageIcon,
  ArrowPathIcon,
  CpuChipIcon,
  HeartIcon,
  SunIcon,
  MoonIcon,
  ChatBubbleLeftRightIcon,
  ShieldExclamationIcon,
  BugAntIcon,
  CircleStackIcon,
  BoltIcon,
  BellAlertIcon,
  MagnifyingGlassIcon,
} from '@heroicons/react/24/outline';
import { useAuthStore } from '@/stores/authStore';
import { useTranslation } from '@/lib/i18n';
import NotificationCenter from '@/components/NotificationCenter';
import OrgSwitcher from '@/components/OrgSwitcher';

/* ── Navigation definitions ── */

const userNavGroups = [
  {
    labelKey: 'nav.group_overview',
    items: [
      { key: 'nav.dashboard', href: '/dashboard', icon: HomeIcon },
      { key: 'nav.playground', href: '/playground', icon: ChatBubbleLeftRightIcon },
    ],
  },
  {
    labelKey: 'nav.group_development',
    items: [
      { key: 'nav.api_keys', href: '/api-keys', icon: KeyIcon },
      { key: 'nav.webhooks', href: '/webhooks', icon: BoltIcon },
      { key: 'nav.data_privacy', href: '/dlp', icon: ShieldCheckIcon },
      { key: 'nav.docs', href: '/docs', icon: DocumentTextIcon },
    ],
  },
  {
    labelKey: 'nav.group_billing',
    items: [
      { key: 'nav.usage', href: '/usage', icon: ChartBarIcon },
      { key: 'nav.subscription', href: '/subscription', icon: CreditCardIcon },
    ],
  },
  {
    labelKey: 'nav.group_account',
    items: [
      { key: 'nav.members', href: '/members', icon: UsersIcon },
      { key: 'nav.profile', href: '/profile', icon: UserIcon },
    ],
  },
];

const adminNavGroups = [
  {
    labelKey: 'nav.group_overview',
    items: [
      { key: 'nav.dashboard', href: '/admin/dashboard', icon: HomeIcon },
      { key: 'nav.analytics', href: '/admin/analytics', icon: ChartBarIcon },
    ],
  },
  {
    labelKey: 'nav.group_users',
    items: [
      { key: 'nav.users', href: '/admin/users', icon: UsersIcon },
      { key: 'nav.announcements', href: '/admin/announcements', icon: MegaphoneIcon },
    ],
  },
  {
    labelKey: 'nav.group_commerce',
    items: [
      { key: 'nav.admin_plans', href: '/admin/plans', icon: SparklesIcon },
      { key: 'nav.promotions', href: '/admin/promotions', icon: TicketIcon },
    ],
  },
  {
    labelKey: 'nav.group_infra',
    items: [
      { key: 'nav.providers', href: '/admin/providers', icon: CpuChipIcon },
      { key: 'nav.routing_engine', href: '/admin/routing-rules', icon: ArrowPathIcon },
      { key: 'nav.proxies', href: '/admin/proxies', icon: GlobeAltIcon },
      { key: 'nav.mcp', href: '/admin/mcp', icon: CommandLineIcon },
      { key: 'nav.cache', href: '/admin/cache', icon: CircleStackIcon },
      { key: 'nav.monitoring', href: '/admin/monitoring', icon: HeartIcon },
      { key: 'nav.admin_notifications', href: '/admin/notifications', icon: BellAlertIcon },
      { key: 'nav.prompts', href: '/admin/prompts', icon: DocumentTextIcon },
      { key: 'nav.rate_limits', href: '/admin/rate-limits', icon: ShieldExclamationIcon },
    ],
  },
  {
    labelKey: 'nav.group_troubleshooting',
    items: [
      { key: 'nav.troubleshooting', href: '/admin/troubleshooting', icon: MagnifyingGlassIcon },
    ],
  },
  {
    labelKey: 'nav.group_system',
    items: [
      { key: 'nav.admin_audit', href: '/admin/audit', icon: ShieldExclamationIcon },
      { key: 'nav.admin_error_logs', href: '/admin/error-logs', icon: BugAntIcon },
      { key: 'nav.admin_docs', href: '/admin/docs', icon: DocumentDuplicateIcon },
      { key: 'nav.admin_settings', href: '/admin/settings', icon: Cog6ToothIcon },
    ],
  },
];

/* ── Shared NavItem renderer ── */

function NavItem({ item, t }: { item: { key: string; href: string; icon: React.ComponentType<React.SVGProps<SVGSVGElement>> }; t: (key: string) => string }) {
  return (
    <NavLink
      to={item.href}
      className={({ isActive }) =>
        clsx(
          'group flex items-center px-4 py-2.5 text-sm font-medium rounded-xl transition-all duration-200',
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
              'mr-3 h-5 w-5 shrink-0 transition-colors'
            )}
            aria-hidden="true"
          />
          <span className="flex-1">{t(item.key)}</span>
        </>
      )}
    </NavLink>
  );
}

/* ── Layout ── */

function Layout() {
  const { user, logout, isAdmin, adminView, toggleAdminView } = useAuthStore();
  const { t, locale, setLocale } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  const [isUserMenuOpen, setIsUserMenuOpen] = useState(false);
  const [isDark, setIsDark] = useState(() => {
    if (typeof window !== 'undefined') {
      return localStorage.getItem('theme') === 'dark';
    }
    return false;
  });
  const userMenuRef = useRef<HTMLDivElement>(null);

  // Load site config
  const { data: siteData } = useQuery<{ siteConfig: { siteName: string } }>(SITE_CONFIG_QUERY);
  const siteName = siteData?.siteConfig?.siteName || 'Router';
  const siteInitial = siteName.charAt(0).toUpperCase();

  const showAdminNav = isAdmin && adminView;

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  const toggleLanguage = () => {
    setLocale(locale === 'en' ? 'zh-CN' : 'en');
  };

  const toggleDarkMode = () => {
    setIsDark((prev) => {
      const next = !prev;
      localStorage.setItem('theme', next ? 'dark' : 'light');
      return next;
    });
  };

  // Apply dark mode class on <html>
  useEffect(() => {
    document.documentElement.classList.toggle('dark', isDark);
  }, [isDark]);

  // Close sidebar on route change (mobile)
  useEffect(() => {
    setIsSidebarOpen(false);
  }, [location.pathname]);

  // Update document title based on current route
  useEffect(() => {
    const allGroups = [...userNavGroups, ...adminNavGroups];
    let routeTitle = '';

    for (const group of allGroups) {
      for (const item of group.items) {
        if (location.pathname === item.href || (item.href !== '/' && location.pathname.startsWith(item.href))) {
          routeTitle = t(item.key);
          break;
        }
      }
      if (routeTitle) break;
    }

    if (routeTitle) {
      document.title = `${routeTitle} - ${siteName}`;
    } else {
      document.title = siteName;
    }
  }, [location.pathname, siteName, t]);

  // If admin switches to user view while on an admin-only page, redirect
  // If admin switches to admin view while on a user page, redirect to admin dashboard
  useEffect(() => {
    if (isAdmin && !adminView && location.pathname.startsWith('/admin')) {
      navigate('/api-keys', { replace: true });
    }
    if (isAdmin && adminView && !location.pathname.startsWith('/admin')) {
      navigate('/admin/dashboard', { replace: true });
    }
  }, [adminView, isAdmin, location.pathname, navigate]);

  // Close user menu on outside click
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (userMenuRef.current && !userMenuRef.current.contains(e.target as Node)) {
        setIsUserMenuOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

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
          'sidebar-bg fixed inset-y-0 left-0 z-50 w-64 backdrop-blur-xl border-r border-apple-gray-200 transform transition-transform duration-300 ease-in-out lg:relative lg:translate-x-0',
          isSidebarOpen ? 'translate-x-0' : '-translate-x-full'
        )}
      >
        <div className="flex flex-col h-full">
          <div className="p-6 pb-3 flex items-center gap-3">
            <div className="w-9 h-9 bg-apple-blue rounded-xl flex items-center justify-center shadow-apple-blue">
              <span className="text-white font-bold text-lg">{siteInitial}</span>
            </div>
            <span className="text-lg font-bold bg-clip-text text-transparent bg-gradient-to-r from-apple-gray-900 to-apple-gray-600">
              {siteName}
            </span>
          </div>

          {/* Organization Switcher */}
          <OrgSwitcher />

          {/* Admin / User View Toggle */}
          {isAdmin && (
            <div className="px-4 pb-3">
              <div className="flex bg-apple-gray-100 rounded-xl p-1 border border-apple-gray-200">
                <button
                  onClick={() => { if (adminView) toggleAdminView(); }}
                  className={clsx(
                    'flex-1 flex items-center justify-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all duration-200',
                    !adminView
                      ? 'bg-apple-gray-50 text-apple-blue shadow-sm border border-apple-gray-200'
                      : 'text-apple-gray-500 hover:text-apple-gray-700'
                  )}
                >
                  <UserIcon className="w-3.5 h-3.5" />
                  {t('nav.user_view')}
                </button>
                <button
                  onClick={() => { if (!adminView) toggleAdminView(); }}
                  className={clsx(
                    'flex-1 flex items-center justify-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all duration-200',
                    adminView
                      ? 'bg-apple-gray-50 text-apple-blue shadow-sm border border-apple-gray-200'
                      : 'text-apple-gray-500 hover:text-apple-gray-700'
                  )}
                >
                  <ShieldCheckIcon className="w-3.5 h-3.5" />
                  {t('nav.admin_view')}
                </button>
              </div>
            </div>
          )}

          <nav className="flex-1 px-3 space-y-1 overflow-y-auto pb-4">
            {(showAdminNav ? adminNavGroups : userNavGroups).map((group, idx) => (
                <div key={group.labelKey} className={idx > 0 ? 'border-t border-apple-gray-100 mt-2 pt-2' : ''}>
                  <div className="pt-3 pb-2 px-4 first:pt-0">
                    <p className="text-[11px] font-semibold text-apple-gray-400 uppercase tracking-wider">
                      {t(group.labelKey)}
                    </p>
                  </div>
                  {group.items.map((item) => (
                    <NavItem key={item.key} item={item} t={t} />
                  ))}
                </div>
              ))}
          </nav>
        </div>
      </aside>

      {/* Main Content */}
      <main className="flex-1 flex flex-col min-w-0 overflow-hidden">
        {/* Top Header */}
        <header className="header-bg backdrop-blur-md border-b border-apple-gray-200 h-14 flex items-center justify-between px-4 lg:px-6 sticky top-0 z-30">
          <div className="flex items-center gap-3">
            <button
              onClick={() => setIsSidebarOpen(true)}
              className="p-2 -ml-2 text-apple-gray-600 lg:hidden"
            >
              <Bars3Icon className="w-5 h-5" />
            </button>
          </div>

          <div className="flex items-center gap-2">
            <NotificationCenter />
            <button
              onClick={toggleDarkMode}
              className="p-2 rounded-lg text-apple-gray-600 hover:text-apple-gray-900 hover:bg-apple-gray-50 transition-colors"
              title={isDark ? 'Switch to light mode' : 'Switch to dark mode'}
            >
              {isDark ? <SunIcon className="w-4.5 h-4.5" /> : <MoonIcon className="w-4.5 h-4.5" />}
            </button>
            <button
              onClick={toggleLanguage}
              className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-apple-gray-600 hover:text-apple-gray-900 hover:bg-apple-gray-50 rounded-lg transition-colors"
              title={locale === 'en' ? '切换为中文' : 'Switch to English'}
            >
              <LanguageIcon className="w-4 h-4" />
              {locale === 'en' ? '中文' : 'EN'}
            </button>

            <div className="relative" ref={userMenuRef}>
              <button
                onClick={() => setIsUserMenuOpen(!isUserMenuOpen)}
                className="flex items-center gap-2.5 pl-3 pr-2 py-1.5 rounded-xl hover:bg-apple-gray-50 transition-colors"
              >
                <div className="hidden sm:block text-right">
                  <p className="text-sm font-medium text-apple-gray-900 leading-tight">
                    {user?.name || 'User'}
                  </p>
                  <p className="text-[11px] text-apple-gray-500 leading-tight">
                    {isAdmin ? 'Admin' : 'User'}
                  </p>
                </div>
                <div className="w-8 h-8 bg-gradient-to-br from-apple-blue to-blue-600 rounded-full flex items-center justify-center text-white font-semibold text-sm shadow-sm">
                  {user?.name?.charAt(0).toUpperCase() || 'U'}
                </div>
              </button>

              <AnimatePresence>
                {isUserMenuOpen && (
                  <motion.div
                    initial={{ opacity: 0, y: -4, scale: 0.95 }}
                    animate={{ opacity: 1, y: 0, scale: 1 }}
                    exit={{ opacity: 0, y: -4, scale: 0.95 }}
                    transition={{ duration: 0.15 }}
                    className="absolute right-0 top-full mt-2 w-56 bg-apple-gray-50 rounded-xl shadow-lg border border-apple-gray-200 py-2 z-50"
                  >
                    <div className="px-4 py-2 border-b border-apple-gray-100">
                      <p className="text-sm font-semibold text-apple-gray-900 truncate">{user?.name || 'User'}</p>
                      <p className="text-xs text-apple-gray-500 truncate">{user?.email}</p>
                    </div>
                    <button
                      onClick={() => { navigate('/profile'); setIsUserMenuOpen(false); }}
                      className="w-full text-left px-4 py-2.5 text-sm text-apple-gray-700 hover:bg-apple-gray-50 flex items-center gap-2.5 transition-colors"
                    >
                      <UserIcon className="w-4 h-4 text-apple-gray-400" />
                      {t('nav.profile')}
                    </button>
                    <div className="border-t border-apple-gray-100 mt-1 pt-1">
                      <button
                        onClick={() => { handleLogout(); setIsUserMenuOpen(false); }}
                        className="w-full text-left px-4 py-2.5 text-sm text-apple-red hover:bg-red-50 flex items-center gap-2.5 transition-colors"
                      >
                        <ArrowRightOnRectangleIcon className="w-4 h-4" />
                        {t('auth.logout')}
                      </button>
                    </div>
                  </motion.div>
                )}
              </AnimatePresence>
            </div>
          </div>
        </header>

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

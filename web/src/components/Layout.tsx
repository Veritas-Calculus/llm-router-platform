import { useState, useEffect } from 'react';
import { Outlet, NavLink, useNavigate, useLocation } from 'react-router-dom';
import { motion, AnimatePresence } from 'framer-motion';
import clsx from 'clsx';
import {
  HomeIcon,
  ChartBarIcon,
  KeyIcon,
  HeartIcon,
  ServerStackIcon,
  GlobeAltIcon,
  Cog6ToothIcon,
  ArrowRightOnRectangleIcon,
  DocumentTextIcon,
  UsersIcon,
  SunIcon,
  MoonIcon,
  ComputerDesktopIcon,
  Bars3Icon,
  XMarkIcon,
} from '@heroicons/react/24/outline';
import { useAuthStore } from '@/stores/authStore';
import { useThemeStore } from '@/stores/themeStore';
import { useTranslation, availableLocales } from '@/lib/i18n';
import { LanguageIcon } from '@heroicons/react/24/outline';
import NotificationCenter from '@/components/NotificationCenter';

const userNavItems = [
  { key: 'nav.dashboard', href: '/dashboard', icon: HomeIcon },
  { key: 'nav.usage', href: '/usage', icon: ChartBarIcon },
  { key: 'nav.api_keys', href: '/api-keys', icon: KeyIcon },
  { key: 'nav.docs', href: '/docs', icon: DocumentTextIcon },
];

const adminNavItems = [
  { key: 'nav.dashboard', href: '/dashboard', icon: HomeIcon },
  { key: 'nav.usage', href: '/usage', icon: ChartBarIcon },
  { key: 'nav.api_keys', href: '/api-keys', icon: KeyIcon },
  // Admin-only sections
  { key: 'nav.users', href: '/users', icon: UsersIcon },
  { key: 'nav.health', href: '/health', icon: HeartIcon },
  { key: 'nav.providers', href: '/providers', icon: ServerStackIcon },
  { key: 'nav.proxies', href: '/proxies', icon: GlobeAltIcon },
  { key: 'nav.settings', href: '/settings', icon: Cog6ToothIcon },
  { key: 'nav.docs', href: '/docs', icon: DocumentTextIcon },
];

function Layout() {
  const navigate = useNavigate();
  const location = useLocation();
  const { user, isAdmin, logout } = useAuthStore();
  const { theme, setTheme } = useThemeStore();
  const { t, locale, setLocale } = useTranslation();
  const [sidebarOpen, setSidebarOpen] = useState(false);

  const navigation = isAdmin ? adminNavItems : userNavItems;

  // Auto-close sidebar on navigation
  useEffect(() => {
    setSidebarOpen(false);
  }, [location.pathname]);

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  const cycleTheme = () => {
    const next = theme === 'light' ? 'dark' : theme === 'dark' ? 'system' : 'light';
    setTheme(next);
  };

  const cycleLang = () => {
    const idx = availableLocales.findIndex((l) => l.code === locale);
    const next = availableLocales[(idx + 1) % availableLocales.length];
    setLocale(next.code);
  };

  const ThemeIcon = theme === 'dark' ? MoonIcon : theme === 'light' ? SunIcon : ComputerDesktopIcon;
  const themeLabel = theme === 'dark' ? t('theme.dark') : theme === 'light' ? t('theme.light') : t('theme.system');
  const currentLangLabel = availableLocales.find((l) => l.code === locale)?.nativeLabel || 'English';

  const sidebarContent = (
    <>
      <div className="h-16 flex items-center justify-between px-6" style={{ borderBottom: '1px solid var(--theme-border-light)' }}>
        <h1 className="text-xl font-semibold" style={{ color: 'var(--theme-text)' }}>LLM Router</h1>
        <div className="flex items-center gap-1">
          {isAdmin && <div className="hidden lg:block"><NotificationCenter /></div>}
          <button
            onClick={() => setSidebarOpen(false)}
            className="lg:hidden p-1 rounded-apple transition-colors"
            style={{ color: 'var(--theme-text-secondary)' }}
          >
            <XMarkIcon className="w-5 h-5" />
          </button>
        </div>
      </div>

      <nav className="flex-1 p-4 space-y-1 overflow-y-auto">
        {navigation.map((item) => (
          <NavLink
            key={item.key}
            to={item.href}
            className={({ isActive }) =>
              clsx(
                'flex items-center gap-3 px-4 py-2.5 rounded-apple transition-colors duration-200',
                isActive
                  ? 'bg-apple-blue text-white'
                  : 'hover:bg-[var(--theme-bg-hover)]'
              )
            }
            style={({ isActive }) => ({
              color: isActive ? 'white' : 'var(--theme-text-secondary)',
            })}
          >
            <item.icon className="w-5 h-5" />
            <span className="font-medium">{t(item.key)}</span>
          </NavLink>
        ))}
      </nav>

      <div className="p-4" style={{ borderTop: '1px solid var(--theme-border-light)' }}>
        <div className="flex items-center gap-3 px-4 py-2 mb-2">
          <div className={clsx(
            "w-8 h-8 rounded-full flex items-center justify-center flex-shrink-0",
            isAdmin ? "bg-amber-500" : "bg-apple-blue"
          )}>
            <span className="text-white text-sm font-medium">
              {user?.name?.charAt(0).toUpperCase() || 'U'}
            </span>
          </div>
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium truncate" style={{ color: 'var(--theme-text)' }}>{user?.name}</p>
            <p className="text-xs truncate" style={{ color: 'var(--theme-text-muted)' }}>
              {isAdmin ? t('users.admin') : t('users.user')} · {user?.email}
            </p>
          </div>
        </div>
        <button
          onClick={cycleLang}
          className="flex items-center gap-3 w-full px-4 py-2.5 rounded-apple transition-colors"
          style={{ color: 'var(--theme-text-secondary)' }}
          title={`Language: ${currentLangLabel}`}
        >
          <LanguageIcon className="w-5 h-5" />
          <span className="font-medium">{currentLangLabel}</span>
        </button>
        <button
          onClick={cycleTheme}
          className="flex items-center gap-3 w-full px-4 py-2.5 rounded-apple transition-colors"
          style={{ color: 'var(--theme-text-secondary)' }}
          title={`Theme: ${themeLabel}`}
        >
          <ThemeIcon className="w-5 h-5" />
          <span className="font-medium">{themeLabel}</span>
        </button>
        <button
          onClick={handleLogout}
          className="flex items-center gap-3 w-full px-4 py-2.5 rounded-apple transition-colors"
          style={{ color: 'var(--theme-text-secondary)' }}
        >
          <ArrowRightOnRectangleIcon className="w-5 h-5" />
          <span className="font-medium">{t('common.sign_out')}</span>
        </button>
      </div>
    </>
  );

  return (
    <div className="flex h-screen bg-apple-gray-50">
      {/* Desktop sidebar — always visible on lg+ */}
      <aside
        className="hidden lg:flex w-64 flex-col flex-shrink-0"
        style={{ backgroundColor: 'var(--theme-bg-sidebar)', borderRight: '1px solid var(--theme-border-light)' }}
      >
        {sidebarContent}
      </aside>

      {/* Mobile sidebar — overlay with backdrop */}
      <AnimatePresence>
        {sidebarOpen && (
          <>
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              className="fixed inset-0 z-40 bg-black/50 lg:hidden"
              onClick={() => setSidebarOpen(false)}
            />
            <motion.aside
              initial={{ x: -256 }}
              animate={{ x: 0 }}
              exit={{ x: -256 }}
              transition={{ type: 'spring', damping: 25, stiffness: 300 }}
              className="fixed inset-y-0 left-0 z-50 w-64 flex flex-col lg:hidden"
              style={{ backgroundColor: 'var(--theme-bg-sidebar)' }}
            >
              {sidebarContent}
            </motion.aside>
          </>
        )}
      </AnimatePresence>

      <main className="flex-1 overflow-auto" style={{ backgroundColor: 'var(--theme-bg)' }}>
        {/* Mobile top bar with hamburger */}
        <div className="lg:hidden flex items-center h-14 px-4 sticky top-0 z-30" style={{ backgroundColor: 'var(--theme-bg-sidebar)', borderBottom: '1px solid var(--theme-border-light)' }}>
          <button
            onClick={() => setSidebarOpen(true)}
            className="p-2 -ml-2 rounded-apple transition-colors"
            style={{ color: 'var(--theme-text)' }}
          >
            <Bars3Icon className="w-6 h-6" />
          </button>
          <h1 className="ml-3 text-lg font-semibold flex-1" style={{ color: 'var(--theme-text)' }}>LLM Router</h1>
          {isAdmin && <NotificationCenter />}
        </div>

        <motion.div
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


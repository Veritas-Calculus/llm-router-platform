import { Outlet, NavLink, useNavigate } from 'react-router-dom';
import { motion } from 'framer-motion';
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
} from '@heroicons/react/24/outline';
import { useAuthStore } from '@/stores/authStore';
import { useThemeStore } from '@/stores/themeStore';
import { useTranslation, availableLocales } from '@/lib/i18n';
import { LanguageIcon } from '@heroicons/react/24/outline';

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
  const { user, isAdmin, logout } = useAuthStore();
  const { theme, setTheme } = useThemeStore();
  const { t, locale, setLocale } = useTranslation();

  const navigation = isAdmin ? adminNavItems : userNavItems;

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

  return (
    <div className="flex h-screen bg-apple-gray-50">
      <aside className="w-64 flex flex-col" style={{ backgroundColor: 'var(--theme-bg-sidebar)', borderRight: '1px solid var(--theme-border-light)' }}>
        <div className="h-16 flex items-center px-6" style={{ borderBottom: '1px solid var(--theme-border-light)' }}>
          <h1 className="text-xl font-semibold" style={{ color: 'var(--theme-text)' }}>LLM Router</h1>
        </div>

        <nav className="flex-1 p-4 space-y-1">
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
              "w-8 h-8 rounded-full flex items-center justify-center",
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
      </aside>

      <main className="flex-1 overflow-auto" style={{ backgroundColor: 'var(--theme-bg)' }}>
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3 }}
          className="p-8"
        >
          <Outlet />
        </motion.div>
      </main>
    </div>
  );
}

export default Layout;

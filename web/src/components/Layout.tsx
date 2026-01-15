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
} from '@heroicons/react/24/outline';
import { useAuthStore } from '@/stores/authStore';

const navigation = [
  { name: 'Dashboard', href: '/dashboard', icon: HomeIcon },
  { name: 'Usage', href: '/usage', icon: ChartBarIcon },
  { name: 'API Keys', href: '/api-keys', icon: KeyIcon },
  { name: 'Health', href: '/health', icon: HeartIcon },
  { name: 'Providers', href: '/providers', icon: ServerStackIcon },
  { name: 'Proxies', href: '/proxies', icon: GlobeAltIcon },
  { name: 'Settings', href: '/settings', icon: Cog6ToothIcon },
  { name: 'Docs', href: '/docs', icon: DocumentTextIcon },
];

function Layout() {
  const navigate = useNavigate();
  const { user, logout } = useAuthStore();

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  return (
    <div className="flex h-screen bg-apple-gray-50">
      <aside className="w-64 bg-white border-r border-apple-gray-200 flex flex-col">
        <div className="h-16 flex items-center px-6 border-b border-apple-gray-200">
          <h1 className="text-xl font-semibold text-apple-gray-900">LLM Router</h1>
        </div>

        <nav className="flex-1 p-4 space-y-1">
          {navigation.map((item) => (
            <NavLink
              key={item.name}
              to={item.href}
              className={({ isActive }) =>
                clsx(
                  'flex items-center gap-3 px-4 py-2.5 rounded-apple transition-colors duration-200',
                  isActive
                    ? 'bg-apple-blue text-white'
                    : 'text-apple-gray-600 hover:bg-apple-gray-100'
                )
              }
            >
              <item.icon className="w-5 h-5" />
              <span className="font-medium">{item.name}</span>
            </NavLink>
          ))}
        </nav>

        <div className="p-4 border-t border-apple-gray-200">
          <div className="flex items-center gap-3 px-4 py-2 mb-2">
            <div className="w-8 h-8 bg-apple-blue rounded-full flex items-center justify-center">
              <span className="text-white text-sm font-medium">
                {user?.name?.charAt(0).toUpperCase() || 'U'}
              </span>
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium text-apple-gray-900 truncate">{user?.name}</p>
              <p className="text-xs text-apple-gray-500 truncate">{user?.email}</p>
            </div>
          </div>
          <button
            onClick={handleLogout}
            className="flex items-center gap-3 w-full px-4 py-2.5 rounded-apple text-apple-gray-600 hover:bg-apple-gray-100 transition-colors"
          >
            <ArrowRightOnRectangleIcon className="w-5 h-5" />
            <span className="font-medium">Sign Out</span>
          </button>
        </div>
      </aside>

      <main className="flex-1 overflow-auto">
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

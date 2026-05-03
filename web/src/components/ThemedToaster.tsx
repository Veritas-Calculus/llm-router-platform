import { Toaster } from 'react-hot-toast';
import { useThemeStore } from '@/stores/themeStore';

export default function ThemedToaster() {
  const isDark = useThemeStore((s) => s.resolvedTheme === 'dark');
  return (
    <Toaster
      position="top-right"
      toastOptions={{
        duration: 4000,
        style: {
          background: isDark ? '#1D1D1F' : '#FFFFFF',
          color: isDark ? '#F5F5F7' : '#1D1D1F',
          border: isDark ? '1px solid rgba(255,255,255,0.1)' : '1px solid #E8E8ED',
          borderRadius: '12px',
          boxShadow: isDark
            ? '0 10px 30px rgba(0,0,0,0.5)'
            : '0 10px 30px rgba(0,0,0,0.08)',
        },
        success: { iconTheme: { primary: '#34C759', secondary: isDark ? '#1D1D1F' : '#FFFFFF' } },
        error: { iconTheme: { primary: '#FF3B30', secondary: isDark ? '#1D1D1F' : '#FFFFFF' } },
      }}
    />
  );
}

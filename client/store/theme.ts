import { create } from 'zustand';
import { Platform } from 'react-native';

export type ThemeScheme = 'light' | 'dark' | 'system';

type ThemeState = {
  colorScheme: ThemeScheme;
  setColorScheme: (scheme: ThemeScheme) => void;
};

// Helper to set data-theme attribute on web
const setWebTheme = (scheme: ThemeScheme) => {
  if (Platform.OS === 'web' && typeof document !== 'undefined') {
    if (scheme === 'system') {
      document.documentElement.removeAttribute('data-theme');
    } else {
      document.documentElement.setAttribute('data-theme', scheme);
    }
  }
};

// Simple store that works on both web and native
export const useThemeStore = create<ThemeState>((set) => ({
  colorScheme: 'system',
  setColorScheme: (scheme) => {
    console.log('[Theme] Changing to:', scheme);
    setWebTheme(scheme);
    set({ colorScheme: scheme });
  },
}));

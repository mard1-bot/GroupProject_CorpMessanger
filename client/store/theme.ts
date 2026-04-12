import { create } from 'zustand';
import { Platform } from 'react-native';

export type ThemeScheme = 'light' | 'dark' | 'system';

type ThemeState = {
  colorScheme: ThemeScheme;
  setColorScheme: (scheme: ThemeScheme) => void;
};

// Simple store that works on both web and native
export const useThemeStore = create<ThemeState>((set) => ({
  colorScheme: 'system',
  setColorScheme: (scheme) => {
    console.log('[Theme] Changing to:', scheme);
    set({ colorScheme: scheme });
  },
}));

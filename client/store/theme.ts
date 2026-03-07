import { create } from 'zustand';

export type ThemeScheme = 'light' | 'dark' | 'system';

type ThemeState = {
  colorScheme: ThemeScheme;
  setColorScheme: (scheme: ThemeScheme) => void;
};

export const useThemeStore = create<ThemeState>((set) => ({
  colorScheme: 'system',
  setColorScheme: (scheme) => set({ colorScheme: scheme }),
}));

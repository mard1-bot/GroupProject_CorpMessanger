import { create } from 'zustand';
import { persist, createJSONStorage } from 'zustand/middleware';
import AsyncStorage from '@react-native-async-storage/async-storage';

export type ThemeScheme = 'light' | 'dark' | 'system';

type ThemeState = {
  colorScheme: ThemeScheme;
  setColorScheme: (scheme: ThemeScheme) => void;
};

export const useThemeStore = create<ThemeState>()(
  persist(
    (set) => ({
      colorScheme: 'system',
      setColorScheme: (scheme) => {
        console.log('[Theme] Changing to:', scheme);
        set({ colorScheme: scheme });
      },
    }),
    {
      name: 'theme-storage',
      storage: createJSONStorage(() => AsyncStorage),
    }
  )
);

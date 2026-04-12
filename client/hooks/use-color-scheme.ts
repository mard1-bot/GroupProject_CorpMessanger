import { useColorScheme as useRNColorScheme, Platform } from 'react-native';
import { useThemeStore } from '@/store/theme';
import { useEffect, useState } from 'react';

export function useColorScheme(): 'light' | 'dark' {
  const systemScheme = useRNColorScheme();
  const preference = useThemeStore((state) => state.colorScheme);
  
  // Web-specific handling for system theme
  const [webSystemScheme, setWebSystemScheme] = useState<'light' | 'dark'>('light');
  
  useEffect(() => {
    if (Platform.OS === 'web' && preference === 'system') {
      const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
      setWebSystemScheme(mediaQuery.matches ? 'dark' : 'light');
      
      const handler = (e: MediaQueryListEvent) => {
        setWebSystemScheme(e.matches ? 'dark' : 'light');
      };
      
      mediaQuery.addEventListener('change', handler);
      return () => mediaQuery.removeEventListener('change', handler);
    }
  }, [preference]);
  
  if (Platform.OS === 'web' && preference === 'system') {
    return webSystemScheme;
  }
  
  const result = preference === 'system' ? (systemScheme ?? 'light') : preference;
  return result;
}

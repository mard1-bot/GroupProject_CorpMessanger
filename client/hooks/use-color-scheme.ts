import { useColorScheme as useRNColorScheme } from 'react-native';
import { useThemeStore } from '@/store/theme';

export function useColorScheme(): 'light' | 'dark' {
  const systemScheme = useRNColorScheme();
  // Use zustand selector - this will cause re-render when colorScheme changes
  const preference = useThemeStore((state) => state.colorScheme);
  
  const result = preference === 'system' ? (systemScheme ?? 'light') : preference;
  return result;
}

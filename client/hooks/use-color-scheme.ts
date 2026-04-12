import { useColorScheme as useRNColorScheme } from 'react-native';
import { useThemeStore } from '@/store/theme';

export function useColorScheme(): 'light' | 'dark' {
  const systemScheme = useRNColorScheme();
  const preference = useThemeStore((s) => s.colorScheme);
  const result = preference === 'system' ? (systemScheme ?? 'light') : preference;
  console.log('[useColorScheme] preference:', preference, 'system:', systemScheme, 'result:', result);
  return result;
}

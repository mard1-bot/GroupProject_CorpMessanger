import { useColorScheme as useRNColorScheme } from 'react-native';
import { useThemeStore } from '@/store/theme';

export function useColorScheme(): 'light' | 'dark' {
  const systemScheme = useRNColorScheme();
  const preference = useThemeStore((s) => s.colorScheme);
  if (preference === 'system') return systemScheme ?? 'light';
  return preference;
}

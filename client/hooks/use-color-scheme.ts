import { useEffect, useState } from 'react';
import { useColorScheme as useRNColorScheme } from 'react-native';
import { useThemeStore } from '@/store/theme';

export function useColorScheme(): 'light' | 'dark' {
  const systemScheme = useRNColorScheme();
  const [forceUpdate, setForceUpdate] = useState(0);
  
  // Subscribe to store changes
  useEffect(() => {
    const unsubscribe = useThemeStore.subscribe((state) => {
      console.log('[useColorScheme] store changed:', state.colorScheme);
      setForceUpdate(v => v + 1);
    });
    return unsubscribe;
  }, []);
  
  const preference = useThemeStore.getState().colorScheme;
  const result = preference === 'system' ? (systemScheme ?? 'light') : preference;
  
  console.log('[useColorScheme] render, forceUpdate:', forceUpdate, 'preference:', preference, 'result:', result);
  
  return result;
}

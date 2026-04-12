import { DarkTheme, DefaultTheme, ThemeProvider } from '@react-navigation/native';
import { Stack } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { useColorScheme as useRNColorScheme, Platform } from 'react-native';
import 'react-native-reanimated';
import { useEffect } from 'react';

import { AuthProvider } from '@/contexts/auth-context';
import { useThemeStore } from '@/store/theme';

// Import CSS for web
if (Platform.OS === 'web') {
  require('@/assets/global.css');
}

export const unstable_settings = {
  anchor: '(tabs)',
};

export default function RootLayout() {
  const systemScheme = useRNColorScheme();
  const preference = useThemeStore((s) => s.colorScheme);
  const colorScheme = preference === 'system' ? (systemScheme ?? 'light') : preference;

  // Initialize web theme attribute on mount
  useEffect(() => {
    if (Platform.OS === 'web' && typeof document !== 'undefined') {
      if (preference !== 'system') {
        document.documentElement.setAttribute('data-theme', preference);
      } else {
        document.documentElement.removeAttribute('data-theme');
      }
    }
  }, [preference]);

  return (
    <AuthProvider>
      <ThemeProvider value={colorScheme === 'dark' ? DarkTheme : DefaultTheme}>
        <Stack>
          <Stack.Screen name="(tabs)" options={{ headerShown: false }} />
          <Stack.Screen name="modal" options={{ presentation: 'modal', title: 'Modal' }} />
        </Stack>
        <StatusBar style="auto" />
      </ThemeProvider>
    </AuthProvider>
  );
}

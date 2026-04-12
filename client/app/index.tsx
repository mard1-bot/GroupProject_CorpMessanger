import { Redirect, useRootNavigationState } from 'expo-router';
import { useAuthStore } from '@/store/auth';

export default function Index() {
  const currentUser = useAuthStore((s) => s.currentUser);
  const rootState = useRootNavigationState();

  if (!rootState?.key) return null;

  if (currentUser) {
    return <Redirect href="/(tabs)" />;
  }
  return <Redirect href="/(auth)/login" />;
}

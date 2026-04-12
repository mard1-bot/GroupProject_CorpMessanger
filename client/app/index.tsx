import { Redirect, useRootNavigationState } from 'expo-router';
import { useAuth } from '@/contexts/auth-context';

export default function Index() {
  const { user: currentUser } = useAuth();
  const rootState = useRootNavigationState();

  if (!rootState?.key) return null;

  if (currentUser) {
    return <Redirect href="/(tabs)" />;
  }
  return <Redirect href="/(auth)/login" />;
}

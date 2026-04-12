import { useEffect, useState } from 'react';
import { StyleSheet, Button, View, ActivityIndicator, Alert } from 'react-native';

import ParallaxScrollView from '@/components/parallax-scroll-view';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { useAuth } from '@/contexts/auth-context';
import { useHealth, useUsers } from '@/hooks/use-api';
import { IconSymbol } from '@/components/ui/icon-symbol';

export default function HomeScreen() {
  const { user, isAuthenticated, logout } = useAuth();
  const { data: healthData, loading: healthLoading, error: healthError, execute: checkHealth } = useHealth();
  const { data: users, loading: usersLoading, execute: loadUsers } = useUsers();

  useEffect(() => {
    checkHealth();
  }, []);

  const handleLogout = async () => {
    await logout();
    Alert.alert('Success', 'Logged out successfully');
  };

  return (
    <ParallaxScrollView
      headerBackgroundColor={{ light: '#A1CEDC', dark: '#1D3D47' }}
      headerImage={
        <IconSymbol
          size={200}
          color="#808080"
          name="bubble.left.and.bubble.right.fill"
          style={styles.headerImage}
        />
      }>
      <ThemedView style={styles.titleContainer}>
        <ThemedText type="title">Corp Messenger</ThemedText>
      </ThemedView>

      <ThemedView style={styles.section}>
        <ThemedText type="subtitle">Backend Connection</ThemedText>
        {healthLoading ? (
          <ActivityIndicator />
        ) : healthError ? (
          <ThemedText style={styles.error}>
            Backend Status: Offline\n{healthError.message}
          </ThemedText>
        ) : healthData ? (
          <ThemedText style={styles.success}>
            Backend Status: {healthData.status}
          </ThemedText>
        ) : null}
        <View style={styles.buttonContainer}>
          <Button title="Check Health" onPress={checkHealth} />
        </View>
      </ThemedView>

      <ThemedView style={styles.section}>
        <ThemedText type="subtitle">Authentication</ThemedText>
        {isAuthenticated ? (
          <>
            <ThemedText style={styles.success}>
              Logged in as: {user?.first_name} {user?.last_name}
            </ThemedText>
            <ThemedText>Email: {user?.email}</ThemedText>
            <View style={styles.buttonContainer}>
              <Button title="Logout" onPress={handleLogout} color="#ff4444" />
            </View>
            <View style={styles.buttonContainer}>
              <Button title="Load Users" onPress={loadUsers} />
            </View>
            {usersLoading ? <ActivityIndicator /> : null}
            {users ? (
              <ThemedText>
                Total users: {users.length}
              </ThemedText>
            ) : null}
          </>
        ) : (
          <>
            <ThemedText style={styles.warning}>
              Not logged in. Go to Login tab to authenticate.
            </ThemedText>
            <ThemedText>
              Backend URL: http://localhost:8080
            </ThemedText>
          </>
        )}
      </ThemedView>

      <ThemedView style={styles.section}>
        <ThemedText type="subtitle">API Endpoints</ThemedText>
        <ThemedText style={styles.code}>
          POST /api/v1/auth/register{'\n'}
          POST /api/v1/auth/login{'\n'}
          GET  /api/v1/auth/me{'\n'}
          GET  /api/v1/users{'\n'}
          POST /api/v1/chats{'\n'}
          GET  /api/v1/chats{'\n'}
          POST /api/v1/chats/:id/messages
        </ThemedText>
      </ThemedView>
    </ParallaxScrollView>
  );
}

const styles = StyleSheet.create({
  titleContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
    marginBottom: 16,
  },
  section: {
    gap: 8,
    marginBottom: 16,
    padding: 12,
    borderRadius: 8,
    borderWidth: 1,
    borderColor: '#ccc',
  },
  headerImage: {
    color: '#808080',
    bottom: -50,
    left: -35,
    position: 'absolute',
  },
  buttonContainer: {
    marginVertical: 8,
  },
  error: {
    color: '#ff4444',
  },
  success: {
    color: '#44ff44',
  },
  warning: {
    color: '#ffaa00',
  },
  code: {
    fontFamily: 'monospace',
    fontSize: 12,
    opacity: 0.8,
  },
});

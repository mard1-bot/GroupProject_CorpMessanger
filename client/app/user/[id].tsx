import { useEffect, useState } from 'react';
import { View, StyleSheet, Image, ActivityIndicator, Alert, TouchableOpacity } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useNavigation } from '@react-navigation/native';
import { MaterialIcons } from '@expo/vector-icons';

import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { useThemeColor } from '@/hooks/use-theme-color';
import { useAuth } from '@/contexts/auth-context';
import { api } from '@/services/api';
import md5 from 'md5';

import { getUserAvatarUrl } from '@/utils/avatar';

export default function UserProfileScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const router = useRouter();
  const navigation = useNavigation();
  const { user: currentUser, isLoading: isAuthLoading } = useAuth();

  const [userData, setUserData] = useState<any>(null);
  const [loading, setLoading] = useState(true);

  const primaryColor = useThemeColor({}, 'primary');
  const textColor = useThemeColor({}, 'text');
  const surfaceColor = useThemeColor({}, 'surface');
  const iconColor = useThemeColor({}, 'icon');

  useEffect(() => {
    navigation.setOptions({
      title: 'Профиль',
      headerShown: true,
      headerLeft: () => (
        <TouchableOpacity onPress={() => router.back()} style={styles.backButton}>
          <MaterialIcons name="arrow-back" size={24} color={primaryColor} />
        </TouchableOpacity>
      ),
    });
  }, [navigation, router, primaryColor]);

  useEffect(() => {
    if (!id || isAuthLoading) return;
    loadUser();
  }, [id, isAuthLoading]);

  const loadUser = async () => {
    try {
      const res = await api.getUserById(id!);
      if (res.data) {
        setUserData(res.data);
      }
    } catch (error) {
      console.error('Failed to load user:', error);
      Alert.alert('Ошибка', 'Не удалось загрузить профиль пользователя');
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return (
      <ThemedView style={styles.centered}>
        <ActivityIndicator size="large" color={primaryColor} />
      </ThemedView>
    );
  }

  if (!userData) {
    return (
      <ThemedView style={styles.centered}>
        <ThemedText>Пользователь не найден</ThemedText>
      </ThemedView>
    );
  }

  const initials = userData.first_name
    ? (userData.first_name[0] + (userData.last_name?.[0] || '')).toUpperCase()
    : '?';

  return (
    <ThemedView style={styles.container}>
      <View style={[styles.header, { backgroundColor: surfaceColor }]}>
        <View style={[styles.avatarLarge, { backgroundColor: primaryColor }]}>
          <Image
            source={{ uri: getUserAvatarUrl(userData, 80) }}
            style={styles.avatarImage}
          />
        </View>
        <ThemedText style={[styles.name, { color: textColor }]}>
          {userData.first_name} {userData.last_name}
        </ThemedText>
        {userData.username && (
          <ThemedText style={[styles.username, { color: iconColor }]}>
            @{userData.username}
          </ThemedText>
        )}
        <ThemedText style={[styles.email, { color: iconColor }]}>
          {userData.email}
        </ThemedText>
      </View>

      <View style={[styles.infoSection, { backgroundColor: surfaceColor }]}>
        <ThemedText style={[styles.sectionTitle, { color: textColor }]}>Информация</ThemedText>
        <View style={styles.infoRow}>
          <MaterialIcons name="email" size={20} color={iconColor} />
          <ThemedText style={[styles.infoText, { color: textColor }]}>
            {userData.email}
          </ThemedText>
        </View>
        <View style={styles.infoRow}>
          <MaterialIcons name="person" size={20} color={iconColor} />
          <ThemedText style={[styles.infoText, { color: textColor }]}>
            {userData.first_name} {userData.last_name}
          </ThemedText>
        </View>
        {userData.username && (
          <View style={styles.infoRow}>
            <MaterialIcons name="alternate-email" size={20} color={iconColor} />
            <ThemedText style={[styles.infoText, { color: textColor }]}>
              @{userData.username}
            </ThemedText>
          </View>
        )}
      </View>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  centered: { flex: 1, justifyContent: 'center', alignItems: 'center' },
  backButton: { paddingHorizontal: 16, paddingVertical: 8 },
  header: {
    alignItems: 'center',
    paddingVertical: 32,
    paddingHorizontal: 16,
  },
  avatarLarge: {
    width: 100,
    height: 100,
    borderRadius: 50,
    justifyContent: 'center',
    alignItems: 'center',
    marginBottom: 16,
    overflow: 'hidden',
  },
  avatarImage: {
    width: 100,
    height: 100,
    borderRadius: 50,
  },
  name: {
    fontSize: 24,
    fontWeight: '700',
    marginBottom: 4,
  },
  username: {
    fontSize: 16,
    marginBottom: 8,
  },
  email: {
    fontSize: 14,
  },
  infoSection: {
    margin: 16,
    padding: 16,
    borderRadius: 12,
  },
  sectionTitle: {
    fontSize: 18,
    fontWeight: '600',
    marginBottom: 16,
  },
  infoRow: {
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: 12,
    gap: 12,
  },
  infoText: {
    fontSize: 16,
  },
});

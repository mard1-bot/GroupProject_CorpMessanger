import { useEffect, useState } from 'react';
import { View, FlatList, TouchableOpacity, StyleSheet, Image } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useNavigation } from '@react-navigation/native';
import { MaterialIcons } from '@expo/vector-icons';

import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { useThemeColor } from '@/hooks/use-theme-color';
import { api, ChatMember } from '@/services/api';
import md5 from 'md5';

const getGravatarUrl = (email: string, size = 48) => {
  const cleanEmail = (email || '').toLowerCase().trim();
  const hash = cleanEmail ? md5(cleanEmail) : '00000000000000000000000000000000';
  return `https://www.gravatar.com/avatar/${hash}?s=${size}&d=mp`;
};

export default function ChatMembersScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const router = useRouter();
  const navigation = useNavigation();
  
  const [members, setMembers] = useState<ChatMember[]>([]);
  const [loading, setLoading] = useState(true);

  const primaryColor = useThemeColor({}, 'primary');
  const textColor = useThemeColor({}, 'text');
  const surfaceColor = useThemeColor({}, 'surface');
  const iconColor = useThemeColor({}, 'icon');

  useEffect(() => {
    navigation.setOptions({
      title: 'Участники',
      headerShown: true,
      headerLeft: () => (
        <TouchableOpacity onPress={() => router.back()} style={styles.backButton}>
          <MaterialIcons name="arrow-back" size={24} color={primaryColor} />
        </TouchableOpacity>
      ),
    });
  }, [navigation, router, primaryColor]);

  useEffect(() => {
    if (!id) return;
    loadMembers();
  }, [id]);

  const loadMembers = async () => {
    try {
      const res = await api.getChatById(id!);
      if (res.data?.members) {
        setMembers(res.data.members);
      }
    } catch (error) {
      console.error('Failed to load members:', error);
    } finally {
      setLoading(false);
    }
  };

  const renderMember = ({ item }: { item: ChatMember }) => {
    const user = item.user;
    if (!user) return null;

    return (
      <View style={[styles.memberItem, { backgroundColor: surfaceColor }]}>
        <Image
          source={{ uri: getGravatarUrl(user.email, 48) }}
          style={styles.avatar}
        />
        <View style={styles.memberInfo}>
          <ThemedText style={styles.name}>
            {user.first_name} {user.last_name}
          </ThemedText>
          <ThemedText style={[styles.username, { color: iconColor }]}>
            @{user.username}
          </ThemedText>
        </View>
        <View style={[styles.roleBadge, { backgroundColor: item.role === 'admin' ? primaryColor : iconColor + '40' }]}>
          <ThemedText style={styles.roleText}>
            {item.role === 'admin' ? 'Админ' : 'Участник'}
          </ThemedText>
        </View>
      </View>
    );
  };

  if (loading) {
    return (
      <ThemedView style={styles.centered}>
        <ThemedText>Загрузка...</ThemedText>
      </ThemedView>
    );
  }

  return (
    <ThemedView style={styles.container}>
      <FlatList
        data={members}
        keyExtractor={(item) => item.user_id}
        renderItem={renderMember}
        contentContainerStyle={styles.list}
        ListEmptyComponent={
          <ThemedText style={styles.empty}>Нет участников</ThemedText>
        }
      />
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  centered: { flex: 1, justifyContent: 'center', alignItems: 'center' },
  backButton: { paddingHorizontal: 16, paddingVertical: 8 },
  list: { padding: 16 },
  memberItem: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: 12,
    borderRadius: 12,
    marginBottom: 8,
  },
  avatar: { width: 48, height: 48, borderRadius: 24 },
  memberInfo: { flex: 1, marginLeft: 12 },
  name: { fontSize: 16, fontWeight: '600' },
  username: { fontSize: 14, marginTop: 2 },
  roleBadge: {
    paddingHorizontal: 10,
    paddingVertical: 4,
    borderRadius: 12,
  },
  roleText: { fontSize: 12, color: '#fff' },
  empty: { textAlign: 'center', marginTop: 40 },
});

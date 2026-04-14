import { useRouter } from 'expo-router';
import dayjs from 'dayjs';
import { useEffect, useMemo, useState } from 'react';
import { FlatList, Pressable, StyleSheet, TextInput, View, ActivityIndicator } from 'react-native';
import MaterialIcons from '@expo/vector-icons/MaterialIcons';
import { SafeAreaView } from 'react-native-safe-area-context';

import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { useThemeColor } from '@/hooks/use-theme-color';
import { useAuth } from '@/contexts/auth-context';
import { api } from '@/services/api';
import type { Chat } from '@/types/chat';

export default function ChatsScreen() {
  const { user: currentUser, isLoading: isAuthLoading } = useAuth();
  const router = useRouter();
  const [searchQuery, setSearchQuery] = useState('');
  const [chats, setChats] = useState<Chat[]>([]);
  const [loading, setLoading] = useState(true);
  const primaryColor = useThemeColor({}, 'primary');
  const textColor = useThemeColor({}, 'text');
  const surfaceColor = useThemeColor({}, 'surface');
  const borderColor = useThemeColor({}, 'border');
  const iconColor = useThemeColor({}, 'icon');
  const bgColor = useThemeColor({}, 'background');

  useEffect(() => {
    if (!isAuthLoading) {
      loadChats();
    }
  }, [isAuthLoading]);

  const loadChats = async () => {
    setLoading(true);
    try {
      const res = await api.getUserChats();
      console.log('Chats loaded:', JSON.stringify(res.data?.[0], null, 2));
      if (res.data) setChats(res.data);
    } catch (error) {
      console.error('Failed to load chats:', error);
    } finally {
      setLoading(false);
    }
  };

  const getOtherParticipant = (chat: Chat, userId: string) => {
    if (chat.type !== 'direct') return null;
    const members = chat.members || [];
    const member = members.find(m => m.user_id !== userId);
    return member?.user || null;
  };

  const getChatDisplayName = (chat: Chat) => {
    if (chat.type === 'direct') {
      const other = currentUser ? getOtherParticipant(chat, currentUser.id) : null;
      if (other) {
        return `${other.first_name} ${other.last_name}`.trim() || other.username || 'Пользователь';
      }
    }
    return chat.title || 'Чат';
  };

  const filteredChats = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return chats;
    return chats.filter((chat) => {
      const other = currentUser ? getOtherParticipant(chat, currentUser.id) : null;
      const name = other ? `${other.first_name} ${other.last_name}`.toLowerCase() : '';
      return name.includes(q);
    });
  }, [chats, currentUser, searchQuery]);

  const renderItem = ({ item: chat }: { item: (typeof filteredChats)[0] }) => {
    const other = currentUser ? getOtherParticipant(chat, currentUser.id) : null;
    const displayName = getChatDisplayName(chat);
    const time = dayjs(chat.updated_at).format('HH:mm');
    const initials = other 
      ? `${other.first_name?.[0] || ''}${other.last_name?.[0] || ''}`.toUpperCase() || other.username?.[0]?.toUpperCase() || '?'
      : (chat.title?.[0]?.toUpperCase() || '?');

    return (
      <Pressable
        onPress={() => router.push(`/chat/${chat.id}`)}
        style={({ pressed }) => [styles.row, pressed && styles.rowPressed]}>
        <View style={[styles.avatar, { backgroundColor: primaryColor }]}>
          <ThemedText style={styles.avatarText}>{initials}</ThemedText>
        </View>
        <View style={styles.content}>
          <View style={styles.rowTop}>
            <ThemedText numberOfLines={1} style={styles.name}>
              {displayName}
            </ThemedText>
            <ThemedText style={styles.time}>{time}</ThemedText>
          </View>
          <ThemedText numberOfLines={1} style={styles.preview}>
            {'Новое сообщение'}
          </ThemedText>
        </View>
      </Pressable>
    );
  };

  return (
    <SafeAreaView style={[styles.safe, { backgroundColor: bgColor }]} edges={['top']}>
      <ThemedView style={styles.container}>
        <View style={styles.headerRow}>
          <ThemedText type="title" style={styles.header}>
            Чаты
          </ThemedText>
          <Pressable
            onPress={() => router.push('/new-chat')}
            style={({ pressed }) => [
              styles.newChatButton,
              pressed && styles.newChatButtonPressed,
              { backgroundColor: primaryColor }
            ]}
          >
            <MaterialIcons name="edit" size={20} color="#fff" />
          </Pressable>
        </View>
        <View style={[styles.searchRow, { backgroundColor: surfaceColor, borderColor }]}>
          <MaterialIcons name="search" size={22} color={iconColor} style={styles.searchIcon} />
          <TextInput
            style={[styles.searchInput, { color: textColor }]}
            placeholder="Поиск"
            placeholderTextColor={iconColor}
            value={searchQuery}
            onChangeText={setSearchQuery}
          />
        </View>
        <FlatList
          data={filteredChats}
          keyExtractor={(item) => item.id}
          renderItem={renderItem}
          contentContainerStyle={styles.list}
          ListEmptyComponent={
            <ThemedText style={styles.empty}>
              {searchQuery.trim() ? 'Ничего не найдено' : 'Нет чатов'}
            </ThemedText>
          }
        />
      </ThemedView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1 },
  container: { flex: 1 },
  header: { paddingTop: 16, paddingBottom: 12 },
  headerRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: 20,
  },
  newChatButton: {
    width: 40,
    height: 40,
    borderRadius: 20,
    justifyContent: 'center',
    alignItems: 'center',
  },
  newChatButtonPressed: { opacity: 0.8 },
  searchRow: {
    flexDirection: 'row',
    alignItems: 'center',
    marginHorizontal: 20,
    marginBottom: 12,
    borderWidth: 1,
    borderRadius: 12,
    paddingHorizontal: 12,
  },
  searchIcon: { marginRight: 8 },
  searchInput: { flex: 1, paddingVertical: 12, fontSize: 16 },
  list: { paddingBottom: 24 },
  row: { flexDirection: 'row', alignItems: 'center', paddingHorizontal: 20, paddingVertical: 14 },
  rowPressed: { opacity: 0.7 },
  avatar: {
    width: 52,
    height: 52,
    borderRadius: 26,
    alignItems: 'center',
    justifyContent: 'center',
    marginRight: 14,
  },
  avatarText: { color: '#fff', fontSize: 18, fontWeight: '600' },
  content: { flex: 1, minWidth: 0 },
  rowTop: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', marginBottom: 2 },
  name: { flex: 1, marginRight: 8, fontWeight: '600' },
  time: { fontSize: 13, opacity: 0.7 },
  preview: { fontSize: 15, opacity: 0.8 },
  empty: { textAlign: 'center', paddingTop: 48, opacity: 0.7 },
});

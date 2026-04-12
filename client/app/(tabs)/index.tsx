import { useRouter } from 'expo-router';
import dayjs from 'dayjs';
import { useMemo, useState } from 'react';
import { FlatList, Pressable, StyleSheet, TextInput, View } from 'react-native';
import MaterialIcons from '@expo/vector-icons/MaterialIcons';
import { SafeAreaView } from 'react-native-safe-area-context';

import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { getOtherParticipant, mockChats } from '@/data/mock';
import { useThemeColor } from '@/hooks/use-theme-color';
import { useAuthStore } from '@/store/auth';

export default function ChatsScreen() {
  const currentUser = useAuthStore((s) => s.currentUser);
  const router = useRouter();
  const [searchQuery, setSearchQuery] = useState('');
  const primaryColor = useThemeColor({}, 'primary');
  const textColor = useThemeColor({}, 'text');
  const surfaceColor = useThemeColor({}, 'surface');
  const borderColor = useThemeColor({}, 'border');
  const iconColor = useThemeColor({}, 'icon');
  const bgColor = useThemeColor({}, 'background');

  const chatsForUser = useMemo(
    () => mockChats.filter((c) => c.participantIds.includes(currentUser?.id ?? '')),
    [currentUser?.id]
  );

  const filteredChats = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return chatsForUser;
    return chatsForUser.filter((chat) => {
      const other = currentUser ? getOtherParticipant(chat, currentUser.id) : null;
      return other?.username?.toLowerCase().includes(q) ?? false;
    });
  }, [chatsForUser, currentUser, searchQuery]);

  const renderItem = ({ item: chat }: { item: (typeof filteredChats)[0] }) => {
    const other = currentUser ? getOtherParticipant(chat, currentUser.id) : null;
    const time = dayjs(chat.updatedAt).format('HH:mm');
    const initials = other?.name ? other.name.split(' ').map((n) => n[0]).join('').slice(0, 2) : '?';

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
              {other?.name ?? 'Чат'}
            </ThemedText>
            <ThemedText style={styles.time}>{time}</ThemedText>
          </View>
          <ThemedText numberOfLines={1} style={styles.preview}>
            {chat.lastMessage ?? ''}
          </ThemedText>
        </View>
      </Pressable>
    );
  };

  return (
    <SafeAreaView style={[styles.safe, { backgroundColor: bgColor }]} edges={['top']}>
      <ThemedView style={styles.container}>
        <ThemedText type="title" style={styles.header}>
          Чаты
        </ThemedText>
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
  header: { paddingHorizontal: 20, paddingTop: 16, paddingBottom: 12 },
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

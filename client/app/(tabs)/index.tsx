import { useRouter } from 'expo-router';
import dayjs from 'dayjs';
import { useEffect, useMemo, useState } from 'react';
import { FlatList, Pressable, StyleSheet, TextInput, View, ActivityIndicator, Platform, Alert, TouchableOpacity } from 'react-native';
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
    let result = chats;
    
    // Filter by search query
    if (q) {
      result = chats.filter((chat) => {
        const other = currentUser ? getOtherParticipant(chat, currentUser.id) : null;
        const name = other ? `${other.first_name} ${other.last_name}`.toLowerCase() : '';
        return name.includes(q);
      });
    }
    
    // Sort: pinned chats first, then by updated_at
    return result.sort((a, b) => {
      const aPinned = a.members?.find(m => m.user_id === currentUser?.id)?.pinned;
      const bPinned = b.members?.find(m => m.user_id === currentUser?.id)?.pinned;
      
      if (aPinned && !bPinned) return -1;
      if (!aPinned && bPinned) return 1;
      
      // Both pinned or both not pinned - sort by updated_at
      return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime();
    });
  }, [chats, currentUser, searchQuery]);

  const handleDeleteChat = async (chatId: string, chatTitle: string) => {
    const confirmDelete = async () => {
      try {
        const res = await api.deleteChat(chatId);
        if (res.data) {
          setChats(prev => prev.filter(c => c.id !== chatId));
          if (Platform.OS === 'web') {
            window.alert('Чат удален');
          } else {
            // Native: no need to alert, visual feedback is enough
          }
        } else if (res.error) {
          const msg = 'Ошибка: ' + res.error.message;
          if (Platform.OS === 'web') {
            window.alert(msg);
          } else {
            Alert.alert('Ошибка', res.error.message);
          }
        }
      } catch (error) {
        if (Platform.OS === 'web') {
          window.alert('Не удалось удалить чат');
        } else {
          Alert.alert('Ошибка', 'Не удалось удалить чат');
        }
      }
    };
    
    if (Platform.OS === 'web') {
      const confirmed = window.confirm(`Удалить чат "${chatTitle}"?\n\nЭто действие нельзя отменить.`);
      if (confirmed) confirmDelete();
    } else {
      Alert.alert(
        'Удалить чат',
        `Удалить "${chatTitle}"?\n\nЭто действие нельзя отменить.`,
        [
          { text: 'Отмена', style: 'cancel' },
          { text: 'Удалить', style: 'destructive', onPress: confirmDelete }
        ]
      );
    }
  };

  const handleLeaveChat = async (chatId: string, chatTitle: string) => {
    const confirmLeave = async () => {
      try {
        const res = await api.removeChatMember(chatId, currentUser!.id);
        if (res.data) {
          setChats(prev => prev.filter(c => c.id !== chatId));
        } else if (res.error) {
          Alert.alert('Ошибка', res.error.message);
        }
      } catch (error) {
        Alert.alert('Ошибка', 'Не удалось выйти из чата');
      }
    };
    
    if (Platform.OS === 'web') {
      const confirmed = window.confirm(`Выйти из чата "${chatTitle}"?`);
      if (confirmed) confirmLeave();
    } else {
      Alert.alert(
        'Выйти из чата',
        `Выйти из "${chatTitle}"?`,
        [
          { text: 'Отмена', style: 'cancel' },
          { text: 'Выйти', style: 'destructive', onPress: confirmLeave }
        ]
      );
    }
  };

  const handleMuteToggle = async (chat: Chat) => {
    const myMember = chat.members?.find(m => m.user_id === currentUser?.id);
    const isMuted = myMember?.muted;
    
    try {
      if (isMuted) {
        const res = await api.unmuteChat(chat.id);
        if (res.data) {
          setChats(prev => prev.map(c => {
            if (c.id === chat.id) {
              return {
                ...c,
                members: c.members?.map(m => 
                  m.user_id === currentUser?.id ? { ...m, muted: false } : m
                )
              };
            }
            return c;
          }));
        }
      } else {
        const res = await api.muteChat(chat.id);
        if (res.data) {
          setChats(prev => prev.map(c => {
            if (c.id === chat.id) {
              return {
                ...c,
                members: c.members?.map(m => 
                  m.user_id === currentUser?.id ? { ...m, muted: true } : m
                )
              };
            }
            return c;
          }));
        }
      }
    } catch (error) {
      Alert.alert('Ошибка', 'Не удалось изменить настройки уведомлений');
    }
  };

  const handlePinToggle = async (chat: Chat) => {
    const myMember = chat.members?.find(m => m.user_id === currentUser?.id);
    const isPinned = myMember?.pinned;
    
    try {
      if (isPinned) {
        const res = await api.unpinChat(chat.id);
        if (res.data) {
          setChats(prev => prev.map(c => {
            if (c.id === chat.id) {
              return {
                ...c,
                members: c.members?.map(m => 
                  m.user_id === currentUser?.id ? { ...m, pinned: false } : m
                )
              };
            }
            return c;
          }));
        }
      } else {
        const res = await api.pinChat(chat.id);
        if (res.data) {
          setChats(prev => prev.map(c => {
            if (c.id === chat.id) {
              return {
                ...c,
                members: c.members?.map(m => 
                  m.user_id === currentUser?.id ? { ...m, pinned: true } : m
                )
              };
            }
            return c;
          }));
        }
      }
    } catch (error) {
      Alert.alert('Ошибка', 'Не удалось закрепить/открепить чат');
    }
  };

  const handleChatMenu = (chat: Chat, displayName: string) => {
    const isOwner = chat.members?.some(m => m.user_id === currentUser?.id && m.role === 'owner');
    const isDirect = chat.type === 'direct';
    const myMember = chat.members?.find(m => m.user_id === currentUser?.id);
    const isMuted = myMember?.muted;
    const isPinned = myMember?.pinned;
    
    if (Platform.OS === 'ios') {
      const { ActionSheetIOS } = require('react-native');
      const muteOption = isMuted ? 'Включить уведомления' : 'Отключить уведомления';
      const pinOption = isPinned ? 'Открепить чат' : 'Закрепить чат';
      const options = isDirect || isOwner 
        ? [pinOption, muteOption, 'Удалить чат', 'Отмена']
        : [pinOption, muteOption, 'Выйти из чата', 'Отмена'];
      const destructiveIndex = isDirect || isOwner ? 2 : 2;
      const cancelIndex = options.length - 1;
      
      ActionSheetIOS.showActionSheetWithOptions(
        { options, destructiveButtonIndex: destructiveIndex, cancelButtonIndex: cancelIndex },
        (buttonIndex: number) => {
          if (buttonIndex === 0) {
            handlePinToggle(chat);
          } else if (buttonIndex === 1) {
            handleMuteToggle(chat);
          } else if (buttonIndex === destructiveIndex) {
            if (isDirect || isOwner) {
              handleDeleteChat(chat.id, displayName);
            } else {
              handleLeaveChat(chat.id, displayName);
            }
          }
        }
      );
    } else {
      const buttons: Array<{text: string, style?: 'default'|'cancel'|'destructive', onPress?: () => void}> = [
        { 
          text: isPinned ? 'Открепить чат' : 'Закрепить чат',
          onPress: () => handlePinToggle(chat)
        },
        { 
          text: isMuted ? 'Включить уведомления' : 'Отключить уведомления',
          onPress: () => handleMuteToggle(chat)
        },
        { 
          text: isDirect || isOwner ? 'Удалить чат' : 'Выйти из чата', 
          style: 'destructive',
          onPress: () => {
            if (isDirect || isOwner) {
              handleDeleteChat(chat.id, displayName);
            } else {
              handleLeaveChat(chat.id, displayName);
            }
          }
        },
        { text: 'Отмена', style: 'cancel' }
      ];
      Alert.alert('Действия', undefined, buttons as any);
    }
  };

  const renderItem = ({ item: chat }: { item: (typeof filteredChats)[0] }) => {
    const other = currentUser ? getOtherParticipant(chat, currentUser.id) : null;
    const displayName = getChatDisplayName(chat);
    const time = dayjs(chat.updated_at).format('HH:mm');
    const initials = other 
      ? `${other.first_name?.[0] || ''}${other.last_name?.[0] || ''}`.toUpperCase() || other.username?.[0]?.toUpperCase() || '?'
      : (chat.title?.[0]?.toUpperCase() || '?');
    const myMember = chat.members?.find(m => m.user_id === currentUser?.id);
    const isMuted = myMember?.muted;
    const isPinned = myMember?.pinned;

    return (
      <View style={styles.rowContainer}>
        <Pressable
          onPress={() => router.push(`/chat/${chat.id}`)}
          {...(Platform.OS === 'web' ? {
            onContextMenu: (e: React.MouseEvent) => {
              e.preventDefault();
              handleDeleteChat(chat.id, displayName);
            }
          } : {})}
          style={({ pressed }) => [styles.row, pressed && styles.rowPressed, isPinned && styles.rowPinned]}>
          <View style={[styles.avatar, { backgroundColor: primaryColor }]}>
            <ThemedText style={styles.avatarText}>{initials}</ThemedText>
          </View>
          <View style={styles.content}>
            <View style={styles.rowTop}>
              <ThemedText numberOfLines={1} style={[styles.name, isPinned && styles.namePinned]}>
                {displayName}
              </ThemedText>
              <View style={styles.timeContainer}>
                {isPinned && (
                  <MaterialIcons name="push-pin" size={14} color={primaryColor} style={styles.pinIcon} />
                )}
                {isMuted && (
                  <MaterialIcons name="notifications-off" size={14} color={iconColor} style={styles.muteIcon} />
                )}
                <ThemedText style={styles.time}>{time}</ThemedText>
              </View>
            </View>
            <ThemedText numberOfLines={1} style={styles.preview}>
              {'Новое сообщение'}
            </ThemedText>
          </View>
        </Pressable>
        {/* Menu button for mobile */}
        <TouchableOpacity 
          onPress={() => handleChatMenu(chat, displayName)}
          style={styles.menuButton}
          hitSlop={{ top: 10, bottom: 10, left: 10, right: 10 }}>
          <MaterialIcons name="more-vert" size={20} color={iconColor} />
        </TouchableOpacity>
      </View>
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
        {loading ? (
          <View style={styles.loadingContainer}>
            <ActivityIndicator size="large" color={primaryColor} />
          </View>
        ) : (
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
        )}
      </ThemedView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1 },
  container: { flex: 1 },
  loadingContainer: { flex: 1, justifyContent: 'center', alignItems: 'center' },
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
  rowContainer: { flexDirection: 'row', alignItems: 'center' },
  row: { flexDirection: 'row', alignItems: 'center', paddingHorizontal: 20, paddingVertical: 14, flex: 1 },
  rowPressed: { opacity: 0.7 },
  menuButton: { paddingHorizontal: 16, paddingVertical: 14, justifyContent: 'center', alignItems: 'center' },
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
  timeContainer: { flexDirection: 'row', alignItems: 'center', gap: 4 },
  muteIcon: { marginRight: 2 },
  pinIcon: { marginRight: 2 },
  rowPinned: { backgroundColor: 'rgba(0,122,255,0.05)' },
  name: { fontSize: 16, fontWeight: '600', flex: 1, marginRight: 8 },
  namePinned: { fontWeight: '700' },
  time: { fontSize: 13, opacity: 0.7 },
  preview: { fontSize: 15, opacity: 0.8 },
  empty: { textAlign: 'center', paddingTop: 48, opacity: 0.7 },
});

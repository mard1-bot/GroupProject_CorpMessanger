import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import {
  View,
  TextInput,
  FlatList,
  TouchableOpacity,
  StyleSheet,
  KeyboardAvoidingView,
  Platform,
  useWindowDimensions,
  Alert,
  Image,
  ActionSheetIOS,
  Modal,
} from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useNavigation } from '@react-navigation/native';
import { MaterialIcons } from '@expo/vector-icons';
import dayjs from 'dayjs';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { useThemeColor } from '@/hooks/use-theme-color';
import { useAuth } from '@/contexts/auth-context';
import { api } from '@/services/api';
import { wsService } from '@/services/websocket';
import { Message, Chat, User } from '@/types/chat';
import md5 from 'md5';

// Gravatar helper with fallback
const getGravatarUrl = (email: string, size = 48) => {
  const cleanEmail = (email || '').toLowerCase().trim();
  const hash = cleanEmail ? md5(cleanEmail) : '00000000000000000000000000000000';
  return `https://www.gravatar.com/avatar/${hash}?s=${size}&d=mp`; // 'mp' = mystery person fallback
};

type ListItem =
  | { type: 'date'; key: string; label: string }
  | { type: 'unread'; key: string; label: string }
  | { type: 'message'; key: string; message: Message };

export default function ChatScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { token, user } = useAuth();
  const router = useRouter();
  const navigation = useNavigation();
  const insets = useSafeAreaInsets();
  const { width } = useWindowDimensions();
  const listRef = useRef<FlatList<ListItem>>(null);

  const [chat, setChat] = useState<Chat | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);
  const [membersMap, setMembersMap] = useState<Record<string, User>>({});
  
  // Edit modal state
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [editingMessage, setEditingMessage] = useState<Message | null>(null);
  const [editText, setEditText] = useState('');

  const isGroup = chat?.type === 'group';

  const primaryColor = useThemeColor({}, 'primary');
  const textColor = useThemeColor({}, 'text');
  const surfaceColor = useThemeColor({}, 'surface');
  const iconColor = useThemeColor({}, 'icon');
  const messageOutgoing = useThemeColor({}, 'messageOutgoing');
  const messageIncoming = useThemeColor({}, 'messageIncoming');
  const borderColor = useThemeColor({}, 'border');

  // Build members map for group chats
  useEffect(() => {
    if (chat?.members) {
      const map: Record<string, User> = {};
      chat.members.forEach(m => {
        if (m.user) {
          map[m.user_id] = m.user;
        }
      });
      console.log('MembersMap built:', Object.keys(map), map);
      setMembersMap(map);
    }
  }, [chat]);

  // Find other participant for direct chat
  const other = useMemo(() => {
    if (!chat || isGroup) return null;
    const member = chat.members?.find(m => m.user_id !== user?.id);
    return member?.user || null;
  }, [chat, user, isGroup]);

  const onMorePress = useCallback(() => {
    if (isGroup) {
      Alert.alert('Меню', undefined, [
        { text: 'Участники', onPress: () => router.push(`/chat/${id}/members`) },
        { text: 'Отмена', style: 'cancel' },
      ]);
    } else {
      Alert.alert('Меню', undefined, [
        { text: 'Найти', onPress: () => {} },
        { text: 'Отмена', style: 'cancel' },
      ]);
    }
  }, [isGroup, id, router]);

  useLayoutEffect(() => {
    const title = isGroup 
      ? (chat?.title || 'Групповой чат')
      : (other ? `${other.first_name} ${other.last_name}` : 'Чат');
    navigation.setOptions({
      title: title,
      headerShown: true,
      headerLeft: () => (
        <TouchableOpacity onPress={() => router.back()} style={styles.headerBackButton} hitSlop={{ top: 12, bottom: 12, left: 12, right: 12 }}>
          <MaterialIcons name="arrow-back" size={24} color={primaryColor} />
        </TouchableOpacity>
      ),
      headerRight: () => (
        <View style={styles.headerRight}>
          <TouchableOpacity onPress={() => {}} style={styles.headerIconButton} hitSlop={8}>
            <MaterialIcons name="videocam" size={24} color={primaryColor} />
          </TouchableOpacity>
          <TouchableOpacity onPress={() => {}} style={styles.headerIconButton} hitSlop={8}>
            <MaterialIcons name="call" size={24} color={primaryColor} />
          </TouchableOpacity>
          <TouchableOpacity onPress={onMorePress} style={styles.headerIconButton} hitSlop={8}>
            <MaterialIcons name="more-vert" size={24} color={primaryColor} />
          </TouchableOpacity>
        </View>
      ),
    });
  }, [navigation, other, chat, isGroup, router, primaryColor, onMorePress]);

  // Fetch chat info
  useEffect(() => {
    if (!id) return;
    console.log('Loading chat:', id);
    api.getChatById(id).then(res => {
      console.log('Chat loaded:', res.data?.id, 'members:', res.data?.members?.length);
      console.log('First member:', res.data?.members?.[0]);
      if (res.data) setChat(res.data);
    }).catch(err => console.error('Failed to load chat:', err));
  }, [id]);

  // Fetch messages
  useEffect(() => {
    if (!id) return;
    api.getChatMessages(id, 50).then(res => {
      console.log('Messages loaded:', res.data?.length, 'First message sender:', res.data?.[0]?.sender_id);
      if (res.data) {
        setMessages(res.data.reverse());
      }
    }).catch(err => console.error('Failed to load messages:', err));
  }, [id]);

  // WebSocket connection
  useEffect(() => {
    if (!token || !id) return;
    wsService.setToken(token);
    wsService.connect();

    const unsubscribe = wsService.onMessage((data: any) => {
      if (data.chat_id !== id) return;
      
      if (data.type === 'message_updated') {
        setMessages(prev => prev.map(m => 
          m.id === data.payload.id 
            ? { ...m, content: data.payload.content, updated_at: data.payload.updated_at }
            : m
        ));
      } else if (data.type === 'message_deleted') {
        setMessages(prev => prev.filter(m => m.id !== data.payload.message_id));
      } else if (data.type === 'read_receipt') {
        // Someone read messages - update read_by for all my messages
        const readerId = data.payload?.reader_id;
        if (readerId) {
          setMessages(prev => prev.map(m => {
            if (m.sender_id === user?.id && !(m.read_by || []).includes(readerId)) {
              return { ...m, read_by: [...(m.read_by || []), readerId] };
            }
            return m;
          }));
        }
      } else if (data.type === 'new_message') {
        const newMessage: Message = {
          id: data.payload.id,
          chat_id: data.payload.chat_id,
          sender_id: data.payload.sender_id,
          type: 'text',
          content: data.payload.content,
          created_at: data.payload.created_at,
          updated_at: data.payload.updated_at || data.payload.created_at,
        };
        setMessages(prev => [...prev, newMessage]);
      }
    });

    return () => {
      unsubscribe();
      wsService.leaveChat(id);
    };
  }, [token, id]);

  // Get last read message ID
  const lastReadMessageId = useMemo(() => {
    for (let i = messages.length - 1; i >= 0; i--) {
      const msg = messages[i];
      if (msg.sender_id === user?.id) {
        const readByOthers = (msg.read_by || []).filter(id => id !== user?.id);
        if (readByOthers.length > 0) {
          return msg.id;
        }
      }
    }
    return null;
  }, [messages, user]);

  const listItems = useMemo(() => {
    const items: ListItem[] = [];
    let lastDate = '';
    let hasShownUnreadDivider = false;
    
    // Find first unread message from others (not from me)
    const firstUnreadIndex = messages.findIndex(m => 
      m.sender_id !== user?.id && !(m.read_by || []).includes(user?.id || '')
    );
    
    messages.forEach((m, index) => {
      const date = dayjs(m.created_at).format('D MMMM YYYY');
      if (date !== lastDate) {
        items.push({ type: 'date', key: `date-${date}`, label: date });
        lastDate = date;
      }
      
      // Show "New messages" divider BEFORE first unread message from others
      if (!hasShownUnreadDivider && index === firstUnreadIndex && firstUnreadIndex !== -1) {
        items.push({ type: 'unread', key: 'unread-divider', label: 'Новые сообщения' });
        hasShownUnreadDivider = true;
      }
      
      items.push({ type: 'message', key: m.id, message: m });
    });
    return items;
  }, [messages, user]);

  const sendMessage = useCallback(async () => {
    const trimmed = input.trim();
    if (!trimmed || !id || !user) return;
    setInput('');
    const tempId = `temp-${Date.now()}`;
    const optimistic: Message = {
      id: tempId,
      chat_id: id,
      sender_id: user.id,
      type: 'text',
      content: trimmed,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      is_failed: false,
    };
    setMessages(prev => [...prev, optimistic]);
    
    try {
      const res = await api.sendMessage(id, trimmed);
      if (res.data) {
        setMessages(prev => prev.map(m => m.id === tempId ? res.data! : m));
      }
    } catch (e) {
      console.error('Send message error:', e);
      // Mark message as failed or remove it
      setMessages(prev => prev.map(m => 
        m.id === tempId ? { ...m, is_failed: true } : m
      ));
      Alert.alert('Ошибка', 'Не удалось отправить сообщение');
    }
  }, [id, user, input]);

  const handleMessageLongPress = useCallback((msg: Message) => {
    const isOwn = msg.sender_id === user?.id;
    console.log('Long press:', { senderId: msg.sender_id, userId: user?.id, isOwn });
    if (!isOwn) return;

    if (Platform.OS === 'ios') {
      ActionSheetIOS.showActionSheetWithOptions(
        {
          options: ['Редактировать', 'Удалить', 'Отмена'],
          destructiveButtonIndex: 1,
          cancelButtonIndex: 2,
        },
        (buttonIndex) => {
          if (buttonIndex === 0) {
            setEditingMessage(msg);
            setEditText(msg.content);
            setEditModalVisible(true);
          } else if (buttonIndex === 1) {
            deleteMessage(msg.id);
          }
        }
      );
    } else {
      Alert.alert('Действия', undefined, [
        { text: 'Редактировать', onPress: () => {
          setEditingMessage(msg);
          setEditText(msg.content);
          setEditModalVisible(true);
        }},
        { text: 'Удалить', style: 'destructive', onPress: () => deleteMessage(msg.id) },
        { text: 'Отмена', style: 'cancel' },
      ]);
    }
  }, [user]);

  const deleteMessage = async (messageId: string) => {
    if (!id) return;
    try {
      await api.deleteMessage(id, messageId);
      setMessages(prev => prev.filter(m => m.id !== messageId));
    } catch (e) {
      console.error('Delete message error:', e);
      Alert.alert('Ошибка', 'Не удалось удалить сообщение');
    }
  };

  const saveEdit = async () => {
    if (!editingMessage || !id || !editText.trim()) return;
    try {
      await api.editMessage(id, editingMessage.id, editText.trim());
      setMessages(prev => prev.map(m => 
        m.id === editingMessage.id 
          ? { ...m, content: editText.trim(), updated_at: new Date().toISOString() }
          : m
      ));
      setEditModalVisible(false);
      setEditingMessage(null);
      setEditText('');
    } catch (e) {
      console.error('Edit message error:', e);
      Alert.alert('Ошибка', 'Не удалось отредактировать сообщение');
    }
  };

  const getSenderInfo = (senderId: string): User | null => {
    if (senderId === user?.id) return user as User;
    const sender = membersMap[senderId];
    console.log('getSenderInfo:', { senderId, found: !!sender, email: sender?.email });
    return sender || null;
  };

  // Status checkmark component - simplified: single = sent, double = read
  const StatusCheckmarks = ({ msg }: { msg: Message }) => {
    const readByOthers = (msg.read_by || []).filter(id => id !== user?.id);
    if (readByOthers.length === 0) {
      // Single checkmark - not read by others yet
      return <MaterialIcons name="check" size={14} color={textColor + '80'} />;
    }
    // Double checkmark - read by at least one other person
    return (
      <View style={styles.doubleCheck}>
        <MaterialIcons name="check" size={14} color={primaryColor} style={{ position: 'absolute', left: 0 }} />
        <MaterialIcons name="check" size={14} color={primaryColor} style={{ position: 'absolute', left: 4 }} />
      </View>
    );
  };

  const renderItem = useCallback(({ item }: { item: ListItem }) => {
    if (item.type === 'date') {
      return (
        <View style={styles.dateRow}>
          <ThemedText style={[styles.dateLabel, { color: iconColor }]}>{item.label}</ThemedText>
        </View>
      );
    }
    if (item.type === 'unread') {
      return (
        <View style={styles.unreadDivider}>
          <View style={[styles.unreadLine, { backgroundColor: primaryColor }]} />
          <ThemedText style={[styles.unreadText, { color: primaryColor }]}>{item.label}</ThemedText>
          <View style={[styles.unreadLine, { backgroundColor: primaryColor }]} />
        </View>
      );
    }
    const msg = item.message;
    const isOwn = msg.sender_id === user?.id;
    const sender = getSenderInfo(msg.sender_id);
    const showAvatar = isGroup && !isOwn && sender;
    const showStatus = isOwn;

    const bubbleContent = (
      <TouchableOpacity 
        onLongPress={() => handleMessageLongPress(msg)}
        activeOpacity={0.9}>
        <View
          style={[
            styles.bubble,
            isOwn ? [styles.bubbleOwn, { backgroundColor: messageOutgoing }] : [styles.bubbleOther, { backgroundColor: messageIncoming }],
          ]}>
          <ThemedText style={[styles.bubbleText, { color: textColor }]}>
            {msg.content}
          </ThemedText>
          <View style={styles.timeRow}>
            <ThemedText style={[styles.time, { color: textColor, opacity: 0.7 }]}>
              {dayjs(msg.created_at).format('HH:mm')}
            </ThemedText>
            {msg.updated_at && msg.updated_at !== msg.created_at && (
              <ThemedText style={[styles.editedLabel, { color: textColor, opacity: 0.5 }]}>
                (изменено)
              </ThemedText>
            )}
            {showStatus && (
              <View style={styles.statusContainer}>
                <StatusCheckmarks msg={msg} />
              </View>
            )}
          </View>
        </View>
      </TouchableOpacity>
    );

    if (!isOwn && isGroup && sender) {
      // Group chat - other user's message: avatar on left, name and bubble stacked vertically
      return (
        <View style={styles.messageRowOtherGroup}>
          <View style={styles.avatarColumn}>
            {sender.email ? (
              <Image
                source={{ uri: getGravatarUrl(sender.email, 40) }}
                style={styles.avatarLarge}
              />
            ) : (
              <View style={[styles.avatarFallbackLarge, { backgroundColor: iconColor + '40' }]}>
                <MaterialIcons name="person" size={20} color={iconColor} />
              </View>
            )}
          </View>
          <View style={styles.messageContentColumn}>
            <ThemedText style={[styles.senderNameAbove, { color: iconColor }]}>
              {sender.first_name || ''} {sender.last_name || ''}
            </ThemedText>
            {bubbleContent}
          </View>
        </View>
      );
    }

    return (
      <View style={[styles.messageRow, isOwn ? styles.messageRowOwn : styles.messageRowOther]}>
        {bubbleContent}
      </View>
    );
  }, [user, membersMap, isGroup, messageOutgoing, messageIncoming, textColor, iconColor, handleMessageLongPress, primaryColor]);

  if (!id || !chat) {
    return (
      <ThemedView style={styles.centered}>
        <ThemedText>Чат не найден</ThemedText>
      </ThemedView>
    );
  }

  return (
    <ThemedView style={styles.container}>
      <KeyboardAvoidingView
        style={styles.keyboard}
        behavior={Platform.OS === 'ios' ? 'padding' : undefined}
        keyboardVerticalOffset={Platform.OS === 'ios' ? 90 : 0}>
        <FlatList
          ref={listRef}
          data={listItems}
          keyExtractor={(item) => item.key}
          renderItem={renderItem}
          inverted={true}
          contentContainerStyle={[styles.list, { paddingTop: insets.bottom + 60 }]}
        />
        <View style={[styles.inputRow, { paddingBottom: insets.bottom + 12, backgroundColor: surfaceColor }]}>
          <TextInput
            style={[styles.input, { color: textColor, backgroundColor: surfaceColor, borderColor }]}
            placeholder="Сообщение"
            placeholderTextColor={iconColor}
            value={input}
            onChangeText={setInput}
            multiline
            maxLength={1000}
          />
          <TouchableOpacity
            style={[styles.sendButton, { backgroundColor: primaryColor }]}
            onPress={sendMessage}
            activeOpacity={0.8}>
            <MaterialIcons name="send" size={22} color="#fff" />
          </TouchableOpacity>
        </View>
      </KeyboardAvoidingView>

      {/* Edit Modal */}
      <Modal
        animationType="slide"
        transparent={true}
        visible={editModalVisible}
        onRequestClose={() => setEditModalVisible(false)}>
        <View style={styles.modalOverlay}>
          <View style={[styles.modalContent, { backgroundColor: surfaceColor }]}>
            <ThemedText style={styles.modalTitle}>Редактировать сообщение</ThemedText>
            <TextInput
              style={[styles.modalInput, { color: textColor, borderColor }]}
              value={editText}
              onChangeText={setEditText}
              multiline
              autoFocus
            />
            <View style={styles.modalButtons}>
              <TouchableOpacity onPress={() => setEditModalVisible(false)} style={styles.modalButton}>
                <ThemedText style={{ color: iconColor }}>Отмена</ThemedText>
              </TouchableOpacity>
              <TouchableOpacity onPress={saveEdit} style={[styles.modalButton, styles.modalButtonPrimary]}>
                <ThemedText style={{ color: '#fff' }}>Сохранить</ThemedText>
              </TouchableOpacity>
            </View>
          </View>
        </View>
      </Modal>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  centered: { flex: 1, justifyContent: 'center', alignItems: 'center' },
  keyboard: { flex: 1 },
  list: { paddingHorizontal: 16, paddingTop: 16 },
  dateRow: { alignItems: 'center', paddingVertical: 12 },
  dateLabel: { fontSize: 13 },
  messageRow: { marginBottom: 12 },
  messageRowOwn: { justifyContent: 'flex-end', flexDirection: 'row' },
  messageRowOther: { justifyContent: 'flex-start', flexDirection: 'row' },
  senderInfo: { flexDirection: 'row', alignItems: 'center', marginBottom: 4, gap: 6 },
  avatar: { width: 24, height: 24, borderRadius: 12 },
  avatarFallback: { width: 24, height: 24, borderRadius: 12, justifyContent: 'center', alignItems: 'center' },
  senderName: { fontSize: 12 },
  bubble: { maxWidth: '75%', minWidth: 140, paddingHorizontal: 14, paddingVertical: 10, borderRadius: 18 },
  bubbleOwn: { borderBottomRightRadius: 4, alignSelf: 'flex-end' },
  bubbleOther: { borderBottomLeftRadius: 4, alignSelf: 'flex-start' },
  messageRowOtherGroup: { flexDirection: 'row', marginBottom: 12, alignItems: 'flex-start' },
  avatarColumn: { marginRight: 8 },
  avatarLarge: { width: 40, height: 40, borderRadius: 20 },
  avatarFallbackLarge: { width: 40, height: 40, borderRadius: 20, justifyContent: 'center', alignItems: 'center' },
  messageContentColumn: { flexDirection: 'column' },
  senderNameAbove: { fontSize: 12, marginBottom: 4 },
  bubbleText: { fontSize: 16 },
  timeRow: { flexDirection: 'row', alignItems: 'center', marginTop: 4, gap: 6 },
  time: { fontSize: 11 },
  editedLabel: { fontSize: 10 },
  inputRow: { flexDirection: 'row', alignItems: 'flex-end', paddingHorizontal: 16, paddingTop: 12, gap: 8 },
  input: {
    flex: 1,
    borderWidth: 1,
    borderRadius: 22,
    paddingHorizontal: 18,
    paddingVertical: 12,
    fontSize: 16,
    maxHeight: 120,
  },
  sendButton: { borderRadius: 22, width: 44, height: 44, alignItems: 'center', justifyContent: 'center' },
  headerBackButton: { paddingHorizontal: 12, paddingVertical: 8, justifyContent: 'center' },
  headerRight: { flexDirection: 'row', alignItems: 'center', gap: 4 },
  headerIconButton: { paddingHorizontal: 8, paddingVertical: 8, justifyContent: 'center' },
  // Status and unread styles
  doubleCheck: { width: 22, height: 14, marginLeft: 2 },
  statusContainer: { marginLeft: 4 },
  unreadDivider: { flexDirection: 'row', alignItems: 'center', marginVertical: 16, paddingHorizontal: 16 },
  unreadLine: { flex: 1, height: 1 },
  unreadText: { fontSize: 12, marginHorizontal: 12, fontWeight: '500' },
  // Modal styles
  modalOverlay: { flex: 1, justifyContent: 'center', alignItems: 'center', backgroundColor: 'rgba(0,0,0,0.5)' },
  modalContent: { width: '85%', borderRadius: 12, padding: 20 },
  modalTitle: { fontSize: 18, fontWeight: '600', marginBottom: 16 },
  modalInput: { borderWidth: 1, borderRadius: 8, padding: 12, fontSize: 16, minHeight: 80, marginBottom: 16 },
  modalButtons: { flexDirection: 'row', justifyContent: 'flex-end', gap: 12 },
  modalButton: { paddingVertical: 10, paddingHorizontal: 16 },
  modalButtonPrimary: { backgroundColor: '#007AFF', borderRadius: 8 },
});

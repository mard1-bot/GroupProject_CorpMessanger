import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import {
  View,
  TextInput,
  FlatList,
  TouchableOpacity,
  Pressable,
  StyleSheet,
  KeyboardAvoidingView,
  Platform,
  useWindowDimensions,
  Alert,
  Image,
  ActionSheetIOS,
  Modal,
  Linking,
  ActivityIndicator,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useNavigation } from '@react-navigation/native';
import { MaterialIcons } from '@expo/vector-icons';
import * as DocumentPicker from 'expo-document-picker';
import dayjs from 'dayjs';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { useThemeColor } from '@/hooks/use-theme-color';
import { useAuth } from '@/contexts/auth-context';
import { CallModal } from '@/components/call-modal';
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
  const [loading, setLoading] = useState(true);
  const [membersMap, setMembersMap] = useState<Record<string, User>>({});
  const [userRole, setUserRole] = useState<string>('');
  
  // Edit modal state
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [editingMessage, setEditingMessage] = useState<Message | null>(null);
  const [editText, setEditText] = useState('');
  const [selectedFile, setSelectedFile] = useState<{ uri: string; name: string; type: string; file?: File | Blob } | null>(null);

  // Search state
  const [searchQuery, setSearchQuery] = useState('');

  // Action menu state (for web)
  const [actionMenuVisible, setActionMenuVisible] = useState(false);
  const [searchResults, setSearchResults] = useState<Message[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [searchVisible, setSearchVisible] = useState(false);

  // Typing indicator state
  const [typingUsers, setTypingUsers] = useState<Set<string>>(new Set());
  const typingTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const typingStopTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const lastTypingSentRef = useRef<number>(0);
  const isTypingActiveRef = useRef<boolean>(false);

  // Call state
  const [callModalVisible, setCallModalVisible] = useState(false);
  const [callType, setCallType] = useState<'audio' | 'video'>('audio');

  // Send typing indicator with debounce (max once per 3 seconds)
  const handleInputChange = useCallback((text: string) => {
    setInput(text);
    
    if (!id || !wsService.isConnected()) return;
    
    const now = Date.now();
    const firstName = user?.first_name;
    const lastName = user?.last_name;
    const trimmedText = text.trim();
    
    // Send typing start if not already active and debounce period passed
    if (!isTypingActiveRef.current && now - lastTypingSentRef.current > 3000) {
      wsService.sendTyping(id, true, firstName, lastName);
      lastTypingSentRef.current = now;
      isTypingActiveRef.current = true;
    }
    
    // Clear existing timers
    if (typingTimerRef.current) {
      clearTimeout(typingTimerRef.current);
      typingTimerRef.current = null;
    }
    if (typingStopTimerRef.current) {
      clearTimeout(typingStopTimerRef.current);
      typingStopTimerRef.current = null;
    }
    
    // Set timer to send typing update when user pauses (if still typing)
    if (trimmedText.length > 0) {
      typingTimerRef.current = setTimeout(() => {
        if (isTypingActiveRef.current) {
          wsService.sendTyping(id, true, firstName, lastName);
          lastTypingSentRef.current = Date.now();
        }
      }, 500);
    }
    
    // Set timer to stop typing indicator when user stops typing
    typingStopTimerRef.current = setTimeout(() => {
      if (isTypingActiveRef.current) {
        wsService.sendTyping(id, false, firstName, lastName);
        isTypingActiveRef.current = false;
      }
    }, 2000);
  }, [id, user]);

  // Cleanup typing timers on unmount or chat change
  useEffect(() => {
    return () => {
      if (typingTimerRef.current) {
        clearTimeout(typingTimerRef.current);
        typingTimerRef.current = null;
      }
      if (typingStopTimerRef.current) {
        clearTimeout(typingStopTimerRef.current);
        typingStopTimerRef.current = null;
      }
      // Send typing stop if still active
      if (isTypingActiveRef.current && id) {
        wsService.sendTyping(id, false, user?.first_name, user?.last_name);
        isTypingActiveRef.current = false;
      }
    };
  }, [id, user]);

  const handleAttachFile = async () => {
    try {
      if (Platform.OS === 'web') {
        // Web: use file input
        const input = document.createElement('input');
        input.type = 'file';
        input.onchange = async (e: any) => {
          const file = e.target.files[0];
          if (file) {
            setSelectedFile({ uri: file.name, name: file.name, type: file.type, file });
          }
        };
        input.click();
      } else {
        // Native: use expo-document-picker
        const result = await DocumentPicker.getDocumentAsync({
          type: '*/*',
          copyToCacheDirectory: true,
        });
        
        if (result.canceled === false && result.assets && result.assets.length > 0) {
          const asset = result.assets[0];
          // Size limit: 50MB
          if (asset.size && asset.size > 50 * 1024 * 1024) {
            Alert.alert('Файл слишком большой', 'Максимальный размер файла 50MB');
            return;
          }
          setSelectedFile({
            uri: asset.uri,
            name: asset.name,
            type: asset.mimeType || 'application/octet-stream',
          });
        }
      }
    } catch (error) {
      console.error('File picker error:', error);
      Alert.alert('Ошибка', 'Не удалось выбрать файл');
    }
  };

  const isGroup = chat?.type === 'group';

  const primaryColor = useThemeColor({}, 'primary');
  const textColor = useThemeColor({}, 'text');
  const surfaceColor = useThemeColor({}, 'surface');
  const iconColor = useThemeColor({}, 'icon');
  const messageOutgoing = useThemeColor({}, 'messageOutgoing');
  const messageIncoming = useThemeColor({}, 'messageIncoming');
  const borderColor = useThemeColor({}, 'border');

  // Build members map for group chats and get current user's role
  useEffect(() => {
    if (chat?.members) {
      const map: Record<string, User> = {};
      let role = '';
      chat.members.forEach(m => {
        if (m.user) {
          map[m.user_id] = m.user;
        }
        if (m.user_id === user?.id && m.role) {
          role = m.role;
        }
      });
      console.log('MembersMap built:', Object.keys(map), map, 'User role:', role);
      setMembersMap(map);
      setUserRole(role);
    }
  }, [chat, user]);

  // Find other participant for direct chat
  const other = useMemo(() => {
    if (!chat || isGroup) return null;
    const member = chat.members?.find(m => m.user_id !== user?.id);
    return member?.user || null;
  }, [chat, user, isGroup]);

  const onMorePress = useCallback(() => {
    console.log('onMorePress called, isGroup:', isGroup, 'chat:', chat);
    if (Platform.OS === 'web') {
      setActionMenuVisible(true);
      return;
    }
    
    // Native: ActionSheet
    const myMember = chat?.members?.find(m => m.user_id === user?.id);
    const isMuted = myMember?.muted;
    const isOwner = myMember?.role === 'owner';
    
    const muteOption = isMuted ? 'Включить уведомления' : 'Отключить уведомления';
    const muteIcon = isMuted ? 'notifications-off' : 'notifications';
    
    if (Platform.OS === 'ios') {
      // Native: ActionSheet
      const { ActionSheetIOS } = require('react-native');
      const options = isGroup
        ? isOwner
          ? [muteOption, 'Удалить чат', 'Отмена']
          : [muteOption, 'Выйти из чата', 'Отмена']
        : [muteOption, 'Удалить чат', 'Отмена'];
      
      const destructiveIndex = isGroup && !isOwner ? 1 : 1;
      const cancelIndex = options.length - 1;
      
      ActionSheetIOS.showActionSheetWithOptions(
        { options, destructiveButtonIndex: destructiveIndex, cancelButtonIndex: cancelIndex },
        (buttonIndex: number) => {
          if (buttonIndex === 0) {
            // Toggle mute
            const action = isMuted ? api.unmuteChat(id!) : api.muteChat(id!);
            action.then(() => {
              setChat(prev => prev ? {
                ...prev,
                members: prev.members?.map(m => 
                  m.user_id === user?.id ? { ...m, muted: !isMuted } : m
                )
              } : null);
              Alert.alert('Готово', isMuted ? 'Уведомления включены' : 'Уведомления отключены');
            }).catch(() => {
              Alert.alert('Ошибка', 'Не удалось изменить настройки');
            });
          } else if (buttonIndex === destructiveIndex) {
            // Delete or leave
            if (isGroup && !isOwner) {
              api.leaveChat(id!).then(() => router.back()).catch(() => {
                Alert.alert('Ошибка', 'Не удалось выйти из чата');
              });
            } else {
              api.deleteChat(id!).then(() => router.back()).catch(() => {
                Alert.alert('Ошибка', 'Не удалось удалить чат');
              });
            }
          }
        }
      );
    }
  }, [isGroup, id, router, chat, user]);

  const onSearchPress = useCallback(() => {
    setSearchVisible(true);
    setSearchQuery('');
    setSearchResults([]);
  }, []);

  const handleSearch = useCallback(async () => {
    if (!searchQuery.trim() || !id) return;
    
    setIsSearching(true);
    try {
      const res = await api.searchMessages(id, searchQuery.trim());
      if (res.data) {
        setSearchResults(res.data);
      } else if (res.error) {
        Alert.alert('Ошибка', res.error.message);
      }
    } catch (error) {
      Alert.alert('Ошибка', 'Не удалось выполнить поиск');
    } finally {
      setIsSearching(false);
    }
  }, [searchQuery, id]);

  const closeSearch = useCallback(() => {
    setSearchVisible(false);
    setSearchQuery('');
    setSearchResults([]);
  }, []);

  const goToMessage = useCallback((messageId: string) => {
    closeSearch();
    // TODO: Scroll to specific message in the future
  }, [closeSearch]);

  useLayoutEffect(() => {
    const title = isGroup 
      ? (chat?.title || 'Групповой чат')
      : (other ? `${other.first_name} ${other.last_name}` : 'Чат');
    navigation.setOptions({
      headerTitle: () => (
        <TouchableOpacity 
          onPress={() => {
            if (isGroup) {
              router.push(`/chat/${id}/members`);
            }
          }}
          disabled={!isGroup}
          style={{ flexDirection: 'row', alignItems: 'center' }}>
          <ThemedText style={{ fontSize: 17, fontWeight: '600' }}>{title}</ThemedText>
          {isGroup && <MaterialIcons name="expand-more" size={20} color={iconColor} style={{ marginLeft: 2 }} />}
        </TouchableOpacity>
      ),
      headerShown: true,
      headerLeft: () => (
        <TouchableOpacity onPress={() => router.back()} style={styles.headerBackButton} hitSlop={{ top: 12, bottom: 12, left: 12, right: 12 }}>
          <MaterialIcons name="arrow-back" size={24} color={primaryColor} />
        </TouchableOpacity>
      ),
      headerRight: () => (
        <View style={styles.headerRight}>
          <TouchableOpacity onPress={onSearchPress} style={styles.headerIconButton} hitSlop={8}>
            <MaterialIcons name="search" size={24} color={primaryColor} />
          </TouchableOpacity>
          <TouchableOpacity 
            onPress={() => {
              setCallType('video');
              setCallModalVisible(true);
            }} 
            style={styles.headerIconButton} 
            hitSlop={8}>
            <MaterialIcons name="videocam" size={24} color={primaryColor} />
          </TouchableOpacity>
          <TouchableOpacity 
            onPress={() => {
              setCallType('audio');
              setCallModalVisible(true);
            }} 
            style={styles.headerIconButton} 
            hitSlop={8}>
            <MaterialIcons name="call" size={24} color={primaryColor} />
          </TouchableOpacity>
          <TouchableOpacity 
            onPress={() => {
              console.log('More button pressed');
              onMorePress();
            }} 
            style={styles.headerIconButton} 
            hitSlop={{ top: 15, bottom: 15, left: 15, right: 15 }}
            activeOpacity={0.6}>
            <MaterialIcons name="more-vert" size={24} color={primaryColor} />
          </TouchableOpacity>
        </View>
      ),
    });
  }, [navigation, other, chat, isGroup, router, primaryColor, onMorePress]);

  // Fetch chat info (wait for token)
  useEffect(() => {
    if (!id || !token) return;
    setLoading(true);
    console.log('[Chat] Loading chat:', id, 'token present:', !!token);
    api.getChatById(id).then(res => {
      console.log('[Chat] Chat loaded:', res.data?.id, 'members:', res.data?.members?.length);
      if (res.data) {
        setChat(res.data);
      } else if (res.error) {
        console.error('[Chat] Failed to load chat:', res.error);
        Alert.alert('Ошибка', 'Не удалось загрузить чат: ' + res.error.message);
      }
    }).catch(err => {
      console.error('[Chat] Exception loading chat:', err);
      Alert.alert('Ошибка', 'Не удалось загрузить чат. Проверьте подключение к интернету.');
    }).finally(() => {
      setLoading(false);
    });
  }, [id, token]);

  // Fetch messages (wait for token)
  useEffect(() => {
    if (!id || !token) return;
    console.log('[Chat] Loading messages for:', id);
    api.getChatMessages(id, 50).then(res => {
      console.log('[Chat] Messages loaded:', res.data?.length, 'error:', res.error?.message);
      if (res.data) {
        setMessages(res.data);
      } else if (res.error) {
        console.error('[Chat] Failed to load messages:', res.error);
      }
    }).catch(err => console.error('[Chat] Exception loading messages:', err));
  }, [id, token]);

  // WebSocket connection
  useEffect(() => {
    if (!token || !id) return;
    wsService.setToken(token);
    wsService.connect();

    // Join chat room after connection is established
    const joinTimer = setTimeout(() => {
      wsService.joinChat(id);
    }, 500);

    const unsubscribe = wsService.onMessage((data: any) => {
      console.log('[Chat] WS message:', data.type, 'chat:', data.chat_id, 'current:', id);
      if (data.chat_id !== id) return;
      
      if (data.type === 'message_updated') {
        setMessages(prev => prev.map(m => 
          m.id === data.payload.id 
            ? { ...m, content: data.payload.content, updated_at: data.payload.updated_at, file_url: data.payload.file_url || m.file_url }
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
        // Remove typing indicator when message arrives
        setTypingUsers(prev => {
          const next = new Set(prev);
          next.delete(newMessage.sender_id);
          return next;
        });
      } else if (data.type === 'typing') {
        console.log('[Chat] Typing event:', data.payload);
        const payload = data.payload;
        const typingUserId = payload?.user_id;
        const isTyping = payload?.is_typing;
        
        if (typingUserId && typingUserId !== user?.id) {
          if (isTyping) {
            setTypingUsers(prev => new Set(prev).add(typingUserId));
          } else {
            setTypingUsers(prev => {
              const next = new Set(prev);
              next.delete(typingUserId);
              return next;
            });
          }
          // Auto-remove after 5 seconds as fallback
          if (isTyping) {
            setTimeout(() => {
              setTypingUsers(prev => {
                const next = new Set(prev);
                next.delete(typingUserId);
                return next;
              });
            }, 5000);
          }
        }
      }
    });

    return () => {
      clearTimeout(joinTimer);
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
    const hasFile = selectedFile !== null;
    
    if ((!trimmed && !hasFile) || !id || !user) return;
    
    // Stop typing indicator
    wsService.sendTyping(id, false, user.first_name, user.last_name);
    
    setInput('');
    const tempId = `temp-${Date.now()}`;
    const optimistic: Message = {
      id: tempId,
      chat_id: id,
      sender_id: user.id,
      type: hasFile ? 'file' : 'text',
      content: trimmed || '[Файл]',
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };
    setMessages(prev => [...prev, optimistic]);
    
    try {
      // Send message first
      const res = await api.sendMessage(id, trimmed || '[Файл]');
      if (!res.data) {
        throw new Error('Failed to send message');
      }
      
      const messageId = res.data.id;
      
      // Upload file if selected
      if (hasFile && messageId && selectedFile) {
        // For web, use the File object directly
        // For mobile, fetch the file from uri
        let fileBlob: Blob | File;
        
        if (Platform.OS === 'web' && selectedFile.file) {
          fileBlob = selectedFile.file;
        } else {
          // Mobile: fetch file from local URI
          const response = await fetch(selectedFile.uri);
          fileBlob = await response.blob();
        }
        
        const uploadRes = await api.uploadFile(id, messageId, fileBlob, selectedFile.name);
        if (uploadRes.data) {
          // Update message with file info
          setMessages(prev => prev.map(m => 
            m.id === tempId ? { ...res.data!, file_url: uploadRes.data!.url } : m
          ));
        }
      } else {
        setMessages(prev => prev.map(m => m.id === tempId ? res.data! : m));
      }
      
      // Clear selected file
      setSelectedFile(null);
    } catch (e) {
      console.error('Send message error:', e);
      setMessages(prev => prev.filter(m => m.id !== tempId));
      Alert.alert('Ошибка', 'Не удалось отправить сообщение');
    }
  }, [id, user, input, selectedFile]);

  const deleteMessage = useCallback(async (messageId: string) => {
    if (!id) return;
    try {
      await api.deleteMessage(id, messageId);
      setMessages(prev => prev.filter(m => m.id !== messageId));
    } catch (e) {
      console.error('Delete message error:', e);
      Alert.alert('Ошибка', 'Не удалось удалить сообщение');
    }
  }, [id]);

  const handleMessageLongPress = useCallback((msg: Message) => {
    const isOwn = msg.sender_id === user?.id;
    // In direct chats, any member can delete any message
    // In group chats, only owner/admin can delete others' messages
    const canDelete = isOwn || !isGroup || userRole === 'owner' || userRole === 'admin';
    const canEdit = isOwn; // Only own messages can be edited
    
    console.log('Long press:', { senderId: msg.sender_id, userId: user?.id, isOwn, isGroup, userRole, canDelete, canEdit });
    
    if (!canDelete && !canEdit) return;

    const options = [];
    if (canEdit) options.push('Редактировать');
    if (canDelete) options.push('Удалить');
    options.push('Отмена');
    
    const editIndex = canEdit ? 0 : -1;
    const deleteIndex = canEdit ? 1 : 0;
    const cancelIndex = options.length - 1;

    if (Platform.OS === 'ios') {
      ActionSheetIOS.showActionSheetWithOptions(
        {
          options,
          destructiveButtonIndex: canDelete ? deleteIndex : undefined,
          cancelButtonIndex: cancelIndex,
        },
        (buttonIndex) => {
          if (canEdit && buttonIndex === editIndex) {
            setEditingMessage(msg);
            setEditText(msg.content);
            setEditModalVisible(true);
          } else if (canDelete && buttonIndex === deleteIndex) {
            deleteMessage(msg.id);
          }
        }
      );
    } else {
      const alertButtons: Array<{
        text: string;
        style?: 'default' | 'cancel' | 'destructive';
        onPress?: () => void;
      }> = [];
      if (canEdit) alertButtons.push({ text: 'Редактировать', onPress: () => {
        setEditingMessage(msg);
        setEditText(msg.content);
        setEditModalVisible(true);
      }});
      if (canDelete) alertButtons.push({ text: 'Удалить', style: 'destructive', onPress: () => deleteMessage(msg.id) });
      alertButtons.push({ text: 'Отмена', style: 'cancel' });
      
      Alert.alert('Действия', undefined, alertButtons as any);
    }
  }, [user, userRole, deleteMessage, isGroup]);

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
    if (senderId === user?.id) return user as unknown as User;
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

    // In direct chats, any member can delete any message
    // In group chats, only owner/admin can delete others' messages
    const canDeleteMessage = isOwn || !isGroup || userRole === 'owner' || userRole === 'admin';
    
    // Debug logging
    console.log('Message render:', { msgId: msg.id, isOwn, isGroup, userRole, canDeleteMessage, senderId: msg.sender_id, userId: user?.id });
    
    const handleContextMenu = (e: any) => {
      e.preventDefault();
      console.log('ContextMenu triggered:', { canDeleteMessage, isOwn, isGroup, userRole });
      if (!canDeleteMessage) return;
      if (typeof window !== 'undefined') {
        const confirmed = window.confirm(isOwn ? 'Удалить это сообщение?' : `Удалить сообщение ${sender ? `${sender.first_name} ${sender.last_name}` : 'пользователя'}?`);
        if (confirmed) {
          deleteMessage(msg.id);
        }
      }
    };

    const contextMenuProps = Platform.OS === 'web' && canDeleteMessage ? { onContextMenu: handleContextMenu } : {};
    
    const handleOpenFile = () => {
      if (!msg.file_url) return;
      const url = api.getFileUrl(msg.file_url);
      if (Platform.OS === 'web') {
        window.open(url, '_blank');
      } else {
        // For mobile, would use Linking
        Linking.openURL(url).catch(() => {
          Alert.alert('Ошибка', 'Не удалось открыть файл');
        });
      }
    };

    const showMenuButton = canDeleteMessage || isOwn;
    
    const bubbleContent = (
      <Pressable
        onLongPress={() => handleMessageLongPress(msg)}
        {...contextMenuProps}
        style={({ pressed }) => [
          styles.bubble,
          isOwn ? [styles.bubbleOwn, { backgroundColor: messageOutgoing }] : [styles.bubbleOther, { backgroundColor: messageIncoming }],
          pressed && { opacity: 0.9 }
        ]}>
        {/* File attachment */}
        {msg.file_url && (
          <TouchableOpacity 
            onPress={handleOpenFile}
            style={[styles.fileAttachment, { backgroundColor: isOwn ? 'rgba(255,255,255,0.2)' : 'rgba(0,0,0,0.05)' }]}>
            <MaterialIcons name="insert-drive-file" size={24} color={textColor} />
            <ThemedText style={[styles.fileName, { color: textColor }]} numberOfLines={1}>
              {msg.file_url.split('/').pop() || 'Файл'}
            </ThemedText>
            <MaterialIcons name="open-in-new" size={18} color={textColor} style={{ opacity: 0.7 }} />
          </TouchableOpacity>
        )}
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
      </Pressable>
    );

    const menuButton = showMenuButton ? (
      <TouchableOpacity
        onPress={() => handleMessageLongPress(msg)}
        style={styles.messageMenuButton}
        hitSlop={{ top: 10, bottom: 10, left: 10, right: 10 }}>
        <MaterialIcons 
          name="more-vert" 
          size={18} 
          color={iconColor} 
          style={{ opacity: 0.5 }}
        />
      </TouchableOpacity>
    ) : null;

    const wrappedContent = (
      <View style={{ flexDirection: 'row', alignItems: 'flex-end' }}>
        {isOwn && menuButton}
        <View {...(Platform.OS === 'web' && canDeleteMessage ? contextMenuProps : {})}>
          {bubbleContent}
        </View>
        {!isOwn && menuButton}
      </View>
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
            {wrappedContent}
          </View>
        </View>
      );
    }

    return (
      <View style={[styles.messageRow, isOwn ? styles.messageRowOwn : styles.messageRowOther]}>
        {wrappedContent}
      </View>
    );
  }, [user, membersMap, isGroup, messageOutgoing, messageIncoming, textColor, iconColor, handleMessageLongPress, primaryColor, userRole]);

  // Show loading state
  if (loading) {
    return (
      <ThemedView style={styles.centered}>
        <ActivityIndicator size="large" color={primaryColor} />
        <ThemedText style={{ marginTop: 12 }}>Загрузка...</ThemedText>
      </ThemedView>
    );
  }

  // Show error only after loading is complete and no chat data
  if (!id || !chat) {
    return (
      <ThemedView style={styles.centered}>
        <MaterialIcons name="error-outline" size={48} color={iconColor} />
        <ThemedText style={{ marginTop: 12 }}>Чат не найден</ThemedText>
        <TouchableOpacity 
          onPress={() => router.back()}
          style={[styles.backButton, { marginTop: 20, backgroundColor: primaryColor, paddingHorizontal: 20, paddingVertical: 10, borderRadius: 8 }]}>
          <ThemedText style={{ color: '#fff' }}>Назад</ThemedText>
        </TouchableOpacity>
      </ThemedView>
    );
  }

  return (
    <ThemedView style={styles.container}>
      <KeyboardAvoidingView
        style={styles.keyboard}
        behavior={Platform.OS === 'ios' ? 'padding' : 'height'}
        keyboardVerticalOffset={Platform.OS === 'ios' ? 90 : 0}>
        
        <FlatList
          ref={listRef}
          data={listItems}
          keyExtractor={(item) => item.key}
          renderItem={renderItem}
          onContentSizeChange={() => listRef.current?.scrollToEnd({ animated: true })}
          onLayout={() => listRef.current?.scrollToEnd({ animated: false })}
          contentContainerStyle={[styles.list, { paddingTop: insets.bottom + 60 }]}
        />
        {selectedFile && (
          <View style={[styles.selectedFileRow, { backgroundColor: surfaceColor }]}>
            <MaterialIcons name="insert-drive-file" size={20} color={primaryColor} />
            <ThemedText style={[styles.selectedFileName, { color: textColor }]} numberOfLines={1}>
              {selectedFile.name}
            </ThemedText>
            <TouchableOpacity onPress={() => { setSelectedFile(null); }}>
              <MaterialIcons name="close" size={20} color={iconColor} />
            </TouchableOpacity>
          </View>
        )}
        {/* Typing indicator */}
        {typingUsers.size > 0 && (
          <View style={[styles.typingContainerBottom, { backgroundColor: surfaceColor }]}>
            <ThemedText style={[styles.typingText, { color: iconColor }]}>
              {typingUsers.size === 1 
                ? `${getSenderInfo(Array.from(typingUsers)[0])?.first_name || 'Кто-то'} печатает...`
                : `${typingUsers.size} человек печатают...`
              }
            </ThemedText>
            <View style={styles.typingDots}>
              <View style={[styles.dot, { backgroundColor: iconColor }, styles.dot1]} />
              <View style={[styles.dot, { backgroundColor: iconColor }, styles.dot2]} />
              <View style={[styles.dot, { backgroundColor: iconColor }, styles.dot3]} />
            </View>
          </View>
        )}
        <View style={[styles.inputRow, { paddingBottom: insets.bottom + 12, backgroundColor: surfaceColor }]}>
          <TouchableOpacity
            style={styles.attachButton}
            onPress={handleAttachFile}
            activeOpacity={0.8}>
            <MaterialIcons name="attach-file" size={24} color={iconColor} />
          </TouchableOpacity>
          <TextInput
            style={[styles.input, { color: textColor, backgroundColor: surfaceColor, borderColor }]}
            placeholder={selectedFile ? "Добавить подпись..." : "Сообщение"}
            placeholderTextColor={iconColor}
            value={input}
            onChangeText={handleInputChange}
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

      {/* Search Modal */}
      <Modal
        animationType="slide"
        transparent={false}
        visible={searchVisible}
        onRequestClose={closeSearch}>
        <SafeAreaView style={[styles.safe, { backgroundColor: surfaceColor }]}>
          <View style={[styles.searchHeader, { backgroundColor: surfaceColor, borderBottomColor: borderColor }]}>
            <TouchableOpacity onPress={closeSearch} style={styles.searchCloseButton}>
              <MaterialIcons name="arrow-back" size={24} color={primaryColor} />
            </TouchableOpacity>
            <TextInput
              style={[styles.searchInput, { color: textColor } ]}
              placeholder="Поиск сообщений..."
              placeholderTextColor={iconColor}
              value={searchQuery}
              onChangeText={setSearchQuery}
              onSubmitEditing={handleSearch}
              autoFocus
            />
            <TouchableOpacity onPress={handleSearch} disabled={isSearching} style={styles.searchButton}>
              {isSearching ? (
                <ActivityIndicator size="small" color={primaryColor} />
              ) : (
                <MaterialIcons name="search" size={24} color={primaryColor} />
              )}
            </TouchableOpacity>
          </View>
          
          <FlatList
            data={searchResults}
            keyExtractor={(item) => item.id}
            renderItem={({ item }) => {
              const sender = getSenderInfo(item.sender_id);
              const isOwn = item.sender_id === user?.id;
              return (
                <TouchableOpacity 
                  style={[styles.searchResultItem, { borderBottomColor: borderColor }]}
                  onPress={() => goToMessage(item.id)}>
                  <View style={styles.searchResultHeader}>
                    <ThemedText style={[styles.searchResultSender, { color: primaryColor }]}>
                      {sender ? `${sender.first_name} ${sender.last_name}` : (isOwn ? 'Вы' : 'Пользователь')}
                    </ThemedText>
                    <ThemedText style={[styles.searchResultTime, { color: iconColor }]}>
                      {dayjs(item.created_at).format('DD.MM.YYYY HH:mm')}
                    </ThemedText>
                  </View>
                  <ThemedText style={[styles.searchResultText, { color: textColor }]} numberOfLines={3}>
                    {item.content}
                  </ThemedText>
                </TouchableOpacity>
              );
            }}
            contentContainerStyle={styles.searchResultsList}
            ListEmptyComponent={
              searchQuery.trim() && !isSearching ? (
                <View style={styles.searchEmpty}>
                  <ThemedText style={{ color: iconColor }}>Ничего не найдено</ThemedText>
                </View>
              ) : null
            }
          />
        </SafeAreaView>
      </Modal>

      {/* Web Action Menu Modal */}
      {Platform.OS === 'web' && (
        <Modal
          animationType="fade"
          transparent={true}
          visible={actionMenuVisible}
          onRequestClose={() => setActionMenuVisible(false)}>
          <TouchableOpacity 
            style={styles.actionMenuOverlay}
            activeOpacity={1}
            onPress={() => setActionMenuVisible(false)}>
            <ThemedView style={[styles.actionMenu, { backgroundColor: surfaceColor }]}>
              {(() => {
                const myMember = chat?.members?.find(m => m.user_id === user?.id);
                const isMuted = myMember?.muted;
                const isOwner = myMember?.role === 'owner';
                return (
                  <>
                    <TouchableOpacity 
                      style={styles.actionMenuItem}
                      onPress={() => {
                        const action = isMuted ? api.unmuteChat(id!) : api.muteChat(id!);
                        action.then(() => {
                          setChat(prev => prev ? {
                            ...prev,
                            members: prev.members?.map(m => 
                              m.user_id === user?.id ? { ...m, muted: !isMuted } : m
                            )
                          } : null);
                          setActionMenuVisible(false);
                        });
                      }}>
                      <MaterialIcons name={isMuted ? 'notifications-off' : 'notifications'} size={20} color={iconColor} />
                      <ThemedText style={styles.actionMenuText}>
                        {isMuted ? 'Включить уведомления' : 'Отключить уведомления'}
                      </ThemedText>
                    </TouchableOpacity>
                    
                    <View style={[styles.actionMenuDivider, { backgroundColor: borderColor }]} />
                    
                    {isGroup && !isOwner ? (
                      <TouchableOpacity 
                        style={[styles.actionMenuItem, styles.actionMenuItemDestructive]}
                        onPress={() => {
                          if (window.confirm('Выйти из чата?')) {
                            api.leaveChat(id!).then(() => {
                              setActionMenuVisible(false);
                              router.back();
                            });
                          }
                        }}>
                        <MaterialIcons name="exit-to-app" size={20} color="#FF3B30" />
                        <ThemedText style={[styles.actionMenuText, { color: '#FF3B30' }]}>
                          Выйти из чата
                        </ThemedText>
                      </TouchableOpacity>
                    ) : (
                      <TouchableOpacity 
                        style={[styles.actionMenuItem, styles.actionMenuItemDestructive]}
                        onPress={() => {
                          if (window.confirm('Удалить чат?')) {
                            api.deleteChat(id!).then(() => {
                              setActionMenuVisible(false);
                              router.back();
                            });
                          }
                        }}>
                        <MaterialIcons name="delete" size={20} color="#FF3B30" />
                        <ThemedText style={[styles.actionMenuText, { color: '#FF3B30' }]}>
                          Удалить чат
                        </ThemedText>
                      </TouchableOpacity>
                    )}
                  </>
                );
              })()}
            </ThemedView>
          </TouchableOpacity>
        </Modal>
      )}

      {/* Call Modal */}
      <CallModal
        visible={callModalVisible}
        onClose={() => setCallModalVisible(false)}
        chatId={id!}
        calleeId={other?.id || chat?.members?.find(m => m.user_id !== user?.id)?.user_id || ''}
        calleeName={other ? `${other.first_name} ${other.last_name}` : 'Пользователь'}
        callType={callType}
      />
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  safe: { flex: 1 },
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
  bubble: Platform.select({
    web: { maxWidth: '75%', minWidth: 140, paddingHorizontal: 14, paddingVertical: 10, borderRadius: 18 },
    default: { maxWidth: '70%', paddingHorizontal: 14, paddingVertical: 10, borderRadius: 18 },
  }),
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
  attachButton: { padding: 8, justifyContent: 'center', alignItems: 'center' },
  selectedFileRow: { flexDirection: 'row', alignItems: 'center', paddingHorizontal: 16, paddingVertical: 8, gap: 8 },
  selectedFileName: { flex: 1, fontSize: 14 },
  fileAttachment: { flexDirection: 'row', alignItems: 'center', padding: 8, borderRadius: 8, marginBottom: 6, gap: 8 },
  fileName: { flex: 1, fontSize: 14 },
  messageMenuButton: { padding: 4, justifyContent: 'center', alignItems: 'center', alignSelf: 'flex-end', marginBottom: 4 },
  backButton: {},
  // Action menu styles (web)
  actionMenuOverlay: { flex: 1, justifyContent: 'center', alignItems: 'center', backgroundColor: 'rgba(0,0,0,0.4)' },
  actionMenu: { width: 280, borderRadius: 12, paddingVertical: 8, ...Platform.select({ web: { boxShadow: '0 2px 4px rgba(0,0,0,0.25)' }, default: { elevation: 5 } }) },
  actionMenuItem: { flexDirection: 'row', alignItems: 'center', paddingHorizontal: 16, paddingVertical: 12, gap: 12 },
  actionMenuItemDestructive: {},
  actionMenuText: { fontSize: 16 },
  actionMenuDivider: { height: 1, marginHorizontal: 0 },
  // Typing indicator styles
  typingContainerBottom: { flexDirection: 'row', alignItems: 'center', paddingHorizontal: 16, paddingVertical: 6, paddingBottom: 2 },
  typingText: { fontSize: 13, marginRight: 8 },
  typingDots: { flexDirection: 'row', gap: 4 },
  dot: { width: 6, height: 6, borderRadius: 3 },
  dot1: { opacity: 0.4 },
  dot2: { opacity: 0.7 },
  dot3: { opacity: 1 },
  // Search styles
  searchHeader: { flexDirection: 'row', alignItems: 'center', paddingHorizontal: 16, paddingVertical: 12, borderBottomWidth: 1 },
  searchCloseButton: { padding: 8, marginRight: 8 },
  searchInput: { flex: 1, fontSize: 16, paddingVertical: 8 },
  searchButton: { padding: 8, marginLeft: 8 },
  searchResultsList: { paddingHorizontal: 16 },
  searchResultItem: { paddingVertical: 12, borderBottomWidth: 1 },
  searchResultHeader: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', marginBottom: 4 },
  searchResultSender: { fontSize: 14, fontWeight: '600' },
  searchResultTime: { fontSize: 12, opacity: 0.7 },
  searchResultText: { fontSize: 15 },
  searchEmpty: { paddingVertical: 48, alignItems: 'center' },
});

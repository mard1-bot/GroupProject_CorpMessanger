import React, { useState, useEffect, useRef, useCallback, useMemo, useLayoutEffect } from 'react';
import {
  View,
  Text,
  TextInput,
  TouchableOpacity,
  StyleSheet,
  KeyboardAvoidingView,
  Platform,
  ScrollView,
  FlatList,
  Alert,
  Modal,
  Image,
  ActivityIndicator,
  useWindowDimensions,
  Linking,
  ActionSheetIOS,
  Pressable,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useNavigation } from '@react-navigation/native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import * as FileSystem from 'expo-file-system';

import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { MentionText } from '@/components/mention-text';
import { useThemeColor } from '@/hooks/use-theme-color';
import { useAuth } from '@/contexts/auth-context';
import { CallModal } from '@/components/call-modal';
import { MessageReactions } from '@/components/message-reactions';
import { MessageActionMenu } from '../../components/message-action-menu';
import { ImagePreview } from '../../components/image-preview';
import { api } from '@/services/api';
import { wsService } from '@/services/websocket';
import { Message, Chat, User } from '@/types/chat';
import md5 from 'md5';
import { MaterialIcons } from '@expo/vector-icons';
import * as DocumentPicker from 'expo-document-picker';
import dayjs from 'dayjs';

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
  const navigation = useNavigation();
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const { user, token } = useAuth();

  const { width } = useWindowDimensions();
  const listRef = useRef<FlatList<ListItem>>(null);

  const [chat, setChat] = useState<Chat | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(true);
  const [membersMap, setMembersMap] = useState<Record<string, User>>({});
  const [userRole, setUserRole] = useState<string>('');
  const [replyingTo, setReplyingTo] = useState<Message | null>(null);
  const [currentPinnedIndex, setCurrentPinnedIndex] = useState(0);
  
  // Edit modal state
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [editingMessage, setEditingMessage] = useState<Message | null>(null);
  const [editText, setEditText] = useState('');
  const [selectedFile, setSelectedFile] = useState<{ uri: string; name: string; type: string; file?: File | Blob } | null>(null);

  // Search state
  const [searchQuery, setSearchQuery] = useState('');

  // Action menu state (for web)
  const [actionMenuVisible, setActionMenuVisible] = useState(false);
  const [messageMenuVisible, setMessageMenuVisible] = useState(false);
  const [selectedMessage, setSelectedMessage] = useState<Message | null>(null);
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

// Failed messages queue for retry
  const [failedMessages, setFailedMessages] = useState<Array<{
    tempId: string;
    content: string;
    type: 'text' | 'file';
    file?: { uri: string; name: string; type: string; file?: File | Blob };
    replyTo?: string;
  }>>([]);

  // Image preview state
  const [imagePreviewVisible, setImagePreviewVisible] = useState(false);
  const [previewImageUrl, setPreviewImageUrl] = useState('');
  const [selectedFilePreview, setSelectedFilePreview] = useState<string | null>(null);

  // Upload progress state
  const [uploadProgress, setUploadProgress] = useState(0);
  const [uploading, setUploading] = useState(false);

  // Forward chat selection state
  const [forwardModalVisible, setForwardModalVisible] = useState(false);
  const [forwardingMessageId, setForwardingMessageId] = useState<string | null>(null);
  const [userChats, setUserChats] = useState<any[]>([]);

  // Load user chats for forwarding
  const loadUserChats = async () => {
    try {
      const response = await api.getUserChats();
      if (response.data) {
        // Filter out current chat and load members for each chat
        const otherChats = response.data.filter((chat: any) => chat.id !== id);
        
        // Load members for each chat to get user names for direct chats
        const chatsWithMembers = await Promise.all(
          otherChats.map(async (chat: any) => {
            try {
              const chatRes = await api.getChatById(chat.id);
              return { ...chat, members: chatRes.data?.members || [] };
            } catch (error) {
              console.error('Failed to load chat members for', chat.id, error);
              return { ...chat, members: [] };
            }
          })
        );
        
        setUserChats(chatsWithMembers);
      }
    } catch (error) {
      console.error('Failed to load chats:', error);
    }
  };

  // Load messages for the chat (using WebSocket for synchronous loading)
  const loadMessages = async () => {
    if (!id || !token) return;
    console.log('[Chat] Loading messages for:', id);
    
    try {
      const res = await api.getChatMessages(id, 50);
      if (res.data) {
        // Load reactions for each message
        const messagesWithReactions = await Promise.all(
          res.data.map(async (msg) => {
            try {
              const reactionsRes = await api.getMessageReactions(msg.id);
              return {
                ...msg,
                reactions: reactionsRes.data || [],
              };
            } catch (error) {
              return msg;
            }
          })
        );
        console.log('[Chat] Messages with reactions loaded:', messagesWithReactions.length);
        setMessages(messagesWithReactions);
      } else if (res.error) {
        console.error('[Chat] Failed to load messages:', res.error);
      }
    } catch (err) {
      console.error('[Chat] Exception loading messages:', err);
    }
  };

  const handleForward = (messageId: string) => {
    console.log('[Chat] Forward message:', messageId);
    setForwardingMessageId(messageId);
    loadUserChats();
    setForwardModalVisible(true);
  };

  const handleExportChat = async () => {
    console.log('[Chat] Export button pressed, id:', id);
    if (!id) {
      console.log('[Chat] Export failed: no id');
      Alert.alert('Ошибка', 'Нет ID чата');
      return;
    }

    console.log('[Chat] Showing export alert');
    
    // Use window.confirm for web, Alert.alert for native
    if (typeof window !== 'undefined' && window.confirm) {
      const confirmed = window.confirm('Экспортировать историю сообщений?');
      if (!confirmed) {
        console.log('[Chat] Export cancelled by user');
        return;
      }
      console.log('[Chat] Export confirmed, calling API');
    } else {
      Alert.alert(
        'Экспорт чата',
        'Экспортировать историю сообщений?',
        [
          { text: 'Отмена', style: 'cancel' },
          {
            text: 'Экспорт',
            onPress: async () => {
              console.log('[Chat] Export confirmed, calling API');
              try {
                const response = await api.exportChat(id, 1000, 0);
                console.log('[Chat] Export API response:', response);
                if (response.data && response.data.messages) {
                  const exportData = {
                    chat_id: response.data.chat_id,
                    export_date: response.data.export_date,
                    messages: response.data.messages,
                    total_messages: response.data.messages.length,
                  };

                  const jsonString = JSON.stringify(exportData, null, 2);

                  if (typeof document !== 'undefined') {
                    // Web: download via blob
                    const blob = new Blob([jsonString], { type: 'application/json' });
                    const url = URL.createObjectURL(blob);
                    const a = document.createElement('a');
                    a.href = url;
                    a.download = `chat_export_${id}_${new Date().toISOString().split('T')[0]}.json`;
                    document.body.appendChild(a);
                    a.click();
                    document.body.removeChild(a);
                    URL.revokeObjectURL(url);
                    Alert.alert('Успех', 'Чат экспортирован');
                  } else {
                    // Mobile: save to file system
                    const fileName = `chat_export_${id}_${new Date().toISOString().split('T')[0]}.json`;
                    const fileUri = (FileSystem as any).documentDirectory + fileName;
                    
                    await FileSystem.writeAsStringAsync(fileUri, jsonString);
                    
                    const fileInfo = await FileSystem.getInfoAsync(fileUri);
                    if (fileInfo.exists) {
                      Alert.alert('Успех', `Чат экспортирован: ${fileUri}`);
                    } else {
                      Alert.alert('Ошибка', 'Не удалось сохранить файл');
                    }
                  }
                } else {
                  Alert.alert('Ошибка', 'Нет данных для экспорта');
                }
              } catch (error) {
                console.error('[Chat] Export error:', error);
                Alert.alert('Ошибка', 'Не удалось экспортировать чат');
              }
            }
          }
        ]
      );
      console.log('[Chat] Export alert shown');
      return;
    }

    // Web export logic
    try {
      const response = await api.exportChat(id, 1000, 0);
      console.log('[Chat] Export API response:', response);
      if (response.data && response.data.messages) {
        const exportData = {
          chat_id: response.data.chat_id,
          export_date: response.data.export_date,
          messages: response.data.messages,
          total_messages: response.data.messages.length,
        };

        const jsonString = JSON.stringify(exportData, null, 2);

        // Web: download via blob
        const blob = new Blob([jsonString], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `chat_export_${id}_${new Date().toISOString().split('T')[0]}.json`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
        Alert.alert('Успех', 'Чат экспортирован');
      } else {
        Alert.alert('Ошибка', 'Нет данных для экспорта');
      }
    } catch (error) {
      console.error('[Chat] Export error:', error);
      Alert.alert('Ошибка', 'Не удалось экспортировать чат');
    }
  };

  const handleThreeDotsPress = useCallback((msg: Message) => {
    setSelectedMessage(msg);
    setActionMenuVisible(true);
  }, []);

  const handleMessageMenuPress = useCallback((msg: Message) => {
    console.log('Message menu button pressed:', msg.id);
    setSelectedMessage(msg);
    setMessageMenuVisible(true);
    console.log('Message menu visible set to true');
  }, []);

  const closeActionMenu = useCallback(() => {
    setActionMenuVisible(false);
    setSelectedMessage(null);
  }, []);

  const closeMessageMenu = useCallback(() => {
    setMessageMenuVisible(false);
    setSelectedMessage(null);
  }, []);

  // Send typing indicator with debounce (max once per 3 seconds)
  const handleInputChange = useCallback((text: string) => {
    setInput(text);
    
    if (!id || !token) return;
    
    const now = Date.now();
    const trimmedText = text.trim();
    
    // Send typing start if not already active and debounce period passed
    if (!isTypingActiveRef.current && now - lastTypingSentRef.current > 3000) {
      api.sendTypingIndicator(id, true).catch(err => {
        console.error('[Chat] Failed to send typing indicator:', err);
      });
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
          api.sendTypingIndicator(id, true).catch(err => {
            console.error('[Chat] Failed to send typing indicator:', err);
          });
          lastTypingSentRef.current = Date.now();
        }
      }, 500);
    }
    
    // Set timer to stop typing indicator when user stops typing
    typingStopTimerRef.current = setTimeout(() => {
      if (isTypingActiveRef.current) {
        api.sendTypingIndicator(id, false).catch(err => {
          console.error('[Chat] Failed to send typing indicator:', err);
        });
        isTypingActiveRef.current = false;
      }
    }, 2000);
  }, [id, token]);

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
        api.sendTypingIndicator(id, false).catch(err => {
          console.error('[Chat] Failed to send typing indicator:', err);
        });
        isTypingActiveRef.current = false;
      }
    };
  }, [id, token]);

  const handleAttachFile = async () => {
    try {
      if (Platform.OS === 'web') {
        // Web: use file input
        const input = document.createElement('input');
        input.type = 'file';
        input.accept = 'image/*,*/*';
        input.onchange = async (e: any) => {
          const file = e.target.files[0];
          if (file) {
            // Create preview URL for images
            let previewUrl: string | null = null;
            if (file.type.startsWith('image/')) {
              previewUrl = URL.createObjectURL(file);
            }
            setSelectedFile({ uri: file.name, name: file.name, type: file.type, file });
            setSelectedFilePreview(previewUrl);
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
          // Use URI as preview for images
          let previewUrl: string | null = null;
          if (asset.mimeType && asset.mimeType.startsWith('image/')) {
            previewUrl = asset.uri;
          }
          setSelectedFile({
            uri: asset.uri,
            name: asset.name,
            type: asset.mimeType || 'application/octet-stream',
          });
          setSelectedFilePreview(previewUrl);
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
      console.log('Opening action menu');
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
          ? [muteOption, 'Экспорт чата', 'Удалить чат', 'Отмена']
          : [muteOption, 'Экспорт чата', 'Выйти из чата', 'Отмена']
        : [muteOption, 'Экспорт чата', 'Удалить чат', 'Отмена'];
      
      const destructiveIndex = isGroup && !isOwner ? 2 : 2;
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
          } else if (buttonIndex === 1) {
            // Export chat
            handleExportChat();
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
    } else if (Platform.OS === 'android') {
      // Android: Use Alert.alert
      const myMember = chat?.members?.find(m => m.user_id === user?.id);
      const isMuted = myMember?.muted;
      const isOwner = myMember?.role === 'owner';
      
      const muteOption = isMuted ? 'Включить уведомления' : 'Отключить уведомления';
      
      const options = isGroup
        ? isOwner
          ? [muteOption, 'Экспорт чата', 'Удалить чат']
          : [muteOption, 'Экспорт чата', 'Выйти из чата']
        : [muteOption, 'Экспорт чата', 'Удалить чат'];
      
      Alert.alert(
        'Действия',
        'Выберите действие',
        options.map((opt) => ({ text: opt, onPress: () => {
          if (opt === muteOption) {
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
          } else if (opt === 'Экспорт чата') {
            handleExportChat();
          } else if (opt === 'Удалить чат' || opt === 'Выйти из чата') {
            if (opt === 'Выйти из чата' && !isOwner) {
              api.leaveChat(id!).then(() => router.back()).catch(() => {
                Alert.alert('Ошибка', 'Не удалось выйти из чата');
              });
            } else {
              api.deleteChat(id!).then(() => router.back()).catch(() => {
                Alert.alert('Ошибка', 'Не удалось удалить чат');
              });
            }
          }
        }}))
      );
    }
  }, [isGroup, id, router, chat, user, handleExportChat]);

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

  // Get all pinned messages
  const pinnedMessages = useMemo(() => messages.filter(m => m.pinned), [messages]);

  // Reset pinned index if out of bounds or no pinned messages
  useEffect(() => {
    if (pinnedMessages.length === 0) {
      setCurrentPinnedIndex(0);
    } else if (currentPinnedIndex >= pinnedMessages.length) {
      setCurrentPinnedIndex(pinnedMessages.length - 1);
    }
  }, [pinnedMessages, currentPinnedIndex]);

  const nextPinnedMessage = useCallback(() => {
    if (pinnedMessages.length === 0) return;
    setCurrentPinnedIndex(prev => (prev + 1) % pinnedMessages.length);
  }, [pinnedMessages.length]);

  const prevPinnedMessage = useCallback(() => {
    if (pinnedMessages.length === 0) return;
    setCurrentPinnedIndex(prev => (prev - 1 + pinnedMessages.length) % pinnedMessages.length);
  }, [pinnedMessages.length]);

  useLayoutEffect(() => {
    const title = isGroup
      ? (chat?.title || 'Групповой чат')
      : (other ? `${other.first_name} ${other.last_name}` : 'Чат');
    
    // Get current pinned message based on index
    const pinnedMessage = pinnedMessages[currentPinnedIndex];
    
    navigation.setOptions({
      headerTitle: () => (
        <View style={{ maxWidth: '100%' }}>
          <TouchableOpacity
            onPress={() => {
              if (isGroup) {
                router.push(`/chat/${id}/members`);
              } else if (other) {
                router.push(`/user/${other.id}`);
              }
            }}>
            <ThemedText style={[styles.headerTitle, { color: textColor }]}>
              {title}
            </ThemedText>
          </TouchableOpacity>
          {pinnedMessage && (
            <View style={{ flexDirection: 'row', alignItems: 'center', marginTop: 2 }}>
              <TouchableOpacity onPress={prevPinnedMessage} hitSlop={8}>
                <MaterialIcons name="chevron-left" size={16} color={iconColor} style={{ marginRight: 4 }} />
              </TouchableOpacity>
              <TouchableOpacity
                onPress={() => {
                  if (pinnedMessage) {
                    scrollToMessage(pinnedMessage.id);
                  }
                }}
                style={{ flexDirection: 'row', alignItems: 'center', flexShrink: 1 }}>
                <MaterialIcons name="push-pin" size={12} color={primaryColor} style={{ marginRight: 4 }} />
                <ThemedText style={{ fontSize: 12, opacity: 0.7 }} numberOfLines={1}>
                  {pinnedMessage.content || 'Вложение'}
                </ThemedText>
                {pinnedMessages.length > 1 && (
                  <ThemedText style={{ fontSize: 12, opacity: 0.5, marginLeft: 4 }}>
                    ({currentPinnedIndex + 1}/{pinnedMessages.length})
                  </ThemedText>
                )}
              </TouchableOpacity>
              <TouchableOpacity onPress={nextPinnedMessage} hitSlop={8}>
                <MaterialIcons name="chevron-right" size={16} color={iconColor} style={{ marginLeft: 4 }} />
              </TouchableOpacity>
            </View>
          )}
        </View>
      ),
      headerShown: true,
      headerLeft: () => (
        <TouchableOpacity onPress={() => router.back()} style={styles.headerBackButton} hitSlop={{ top: 12, bottom: 12, left: 12, right: 12 }}>
          <MaterialIcons name="arrow-back" size={24} color={primaryColor} />
        </TouchableOpacity>
      ),
      headerRight: () => (
        <View style={[styles.headerRight, { zIndex: 10 }]} pointerEvents="box-none">
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
          {isGroup && (
            <TouchableOpacity
              onPress={() => {
                router.push(`/chat/${id}/members`);
              }}
              style={styles.headerIconButton}
              hitSlop={8}>
              <MaterialIcons name="group" size={24} color={primaryColor} />
            </TouchableOpacity>
          )}
          <TouchableOpacity
            onPress={onMorePress}
            style={styles.headerIconButton}
            hitSlop={8}>
            <MaterialIcons name="more-vert" size={24} color={primaryColor} />
          </TouchableOpacity>
        </View>
      ),
    });
  }, [navigation, isGroup, chat, other, iconColor, primaryColor, pinnedMessages, currentPinnedIndex, nextPinnedMessage, prevPinnedMessage, onSearchPress]);

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
    loadMessages();
  }, [id, token]);

  // WebSocket connection will be added after retryFailedMessages function

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

  const scrollToMessage = useCallback((messageId: string) => {
    const listIndex = listItems.findIndex(item => item.type === 'message' && item.message?.id === messageId);
    if (listIndex !== -1 && listRef.current) {
      try {
        listRef.current?.scrollToIndex({ index: listIndex, animated: true, viewPosition: 0.5 });
      } catch (e) {
        listRef.current?.scrollToEnd({ animated: true });
      }
    }
  }, [listItems]);

  const scrollToCurrentPinned = useCallback(() => {
    if (pinnedMessages[currentPinnedIndex]) {
      scrollToMessage(pinnedMessages[currentPinnedIndex].id);
    }
  }, [currentPinnedIndex, pinnedMessages, scrollToMessage]);

  const sendMessage = useCallback(async () => {
    const trimmed = input.trim();
    const hasFile = selectedFile !== null;

    // Валидация пустого сообщения
    if (!trimmed && !hasFile) {
      Alert.alert('Ошибка', 'Сообщение не может быть пустым');
      return;
    }

    // Проверка формата файла
    if (hasFile && selectedFile) {
      const allowedTypes = [
        'image/jpeg', 'image/png', 'image/gif', 'image/webp',
        'application/pdf', 'text/plain', 'application/msword',
        'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
        'application/vnd.ms-excel', 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
        'video/mp4', 'video/avi', 'video/mov'
      ];
      
      const fileExtension = selectedFile.name.split('.').pop()?.toLowerCase();
      const allowedExtensions = ['jpg', 'jpeg', 'png', 'gif', 'webp', 'pdf', 'txt', 'doc', 'docx', 'xls', 'xlsx', 'mp4', 'avi', 'mov'];
      
      if (fileExtension && !allowedExtensions.includes(fileExtension)) {
        Alert.alert('Ошибка', `Файл формата .${fileExtension} не поддерживается. Поддерживаемые форматы: ${allowedExtensions.join(', ')}`);
        return;
      }
    }

    if (!id || !user) {
      Alert.alert('Ошибка', 'Не удалось определить чат или пользователя');
      return;
    }

    // Проверка соединения
    if (!wsService.isConnected()) {
      Alert.alert(
        'Отсутствует соединение',
        'Нет подключения к серверу. Сообщение будет отправлено автоматически при восстановлении соединения.',
        [
          { text: 'OK', style: 'default' },
          { text: 'Повторить', onPress: () => sendMessage() }
        ]
      );
      return;
    }

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
      reply_to: replyingTo?.id,
      status: 'sending', // Добавляем статус отправки
    };
    setMessages(prev => [...prev, optimistic]);

    try {
      // Send message first
      const res = await api.sendMessage(id, trimmed || '[Файл]', hasFile ? 'file' : 'text', replyingTo?.id);
      if (!res.data) {
        throw new Error('Failed to send message');
      }
      
      const messageId = res.data.id;
      
      // Update message status to sent
      setMessages(prev => prev.map(m => 
        m.id === tempId ? { ...m, id: res.data!.id, status: 'sent' } : m
      ));
      
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
            m.id === res.data!.id ? { ...res.data!, file_url: uploadRes.data!.url, status: 'sent' } : m
          ));
        }
      } else {
        setMessages(prev => prev.map(m => m.id === res.data!.id ? { ...res.data!, status: 'sent' } : m));
      }

      // Auto-scroll after message is confirmed
      setTimeout(() => {
        listRef.current?.scrollToEnd({ animated: true });
      }, 100);

      // Clear selected file and reply state
      setSelectedFile(null);
      setReplyingTo(null);
    } catch (e) {
      console.error('Send message error:', e);
      
      // Update message status to failed
      setMessages(prev => prev.map(m => 
        m.id === tempId ? { ...m, status: 'failed' } : m
      ));
      
      // Add to failed messages queue for retry
      setFailedMessages(prev => [...prev, {
        tempId,
        content: trimmed,
        type: hasFile ? 'file' : 'text',
        file: hasFile ? selectedFile : undefined,
        replyTo: replyingTo?.id
      }]);
      
      Alert.alert(
        'Ошибка отправки',
        'Не удалось отправить сообщение. Сообщение будет автоматически отправлено при восстановлении соединения.',
        [
          { text: 'OK', style: 'default' },
          { text: 'Повторить', onPress: () => {
            // Remove failed message and try again immediately
            setMessages(prev => prev.filter(m => m.id !== tempId));
            setFailedMessages(prev => prev.filter(msg => msg.tempId !== tempId));
            setInput(trimmed);
            if (hasFile) setSelectedFile(selectedFile);
            setReplyingTo(replyingTo);
          }}
        ]
      );
    }
  }, [id, user, input, selectedFile, replyingTo]);

  // Retry failed messages when connection is restored
  const retryFailedMessages = useCallback(async () => {
    if (failedMessages.length === 0) return;
    
    console.log('[Chat] Retrying', failedMessages.length, 'failed messages');
    
    for (const failedMsg of failedMessages) {
      try {
        // Remove the failed message from UI first
        setMessages(prev => prev.filter(m => m.id !== failedMsg.tempId));
        
        // Create new optimistic message
        const newTempId = `retry-${Date.now()}-${Math.random()}`;
        const optimistic: Message = {
          id: newTempId,
          chat_id: id!,
          sender_id: user!.id,
          type: failedMsg.type,
          content: failedMsg.content,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
          reply_to: failedMsg.replyTo,
          status: 'sending',
        };
        setMessages(prev => [...prev, optimistic]);
        
        // Send message
        const res = await api.sendMessage(id!, failedMsg.content, failedMsg.type, failedMsg.replyTo);
        if (!res.data) {
          throw new Error('Failed to send message');
        }
        
        // Update message status to sent
        setMessages(prev => prev.map(m => 
          m.id === newTempId ? { ...m, id: res.data!.id, status: 'sent' } : m
        ));
        
        // Upload file if needed
        if (failedMsg.type === 'file' && failedMsg.file && res.data.id) {
          let fileBlob: Blob | File;
          
          if (Platform.OS === 'web' && failedMsg.file.file) {
            fileBlob = failedMsg.file.file;
          } else {
            const response = await fetch(failedMsg.file.uri);
            fileBlob = await response.blob();
          }
          
          const uploadRes = await api.uploadFile(id!, res.data.id, fileBlob, failedMsg.file.name);
          if (uploadRes.data) {
            setMessages(prev => prev.map(m => 
              m.id === res.data!.id ? { ...res.data!, file_url: uploadRes.data!.url, status: 'sent' } : m
            ));
          }
        }
        
        // Remove from failed messages queue
        setFailedMessages(prev => prev.filter(msg => msg.tempId !== failedMsg.tempId));
        
      } catch (e) {
        console.error('Retry message error:', e);
        // Update status to failed again
        setMessages(prev => prev.map(m => 
          m.id.startsWith('retry-') ? { ...m, status: 'failed' } : m
        ));
      }
    }
  }, [failedMessages, id, user]);

  // WebSocket connection
  useEffect(() => {
    if (!token || !id) return;
    wsService.setToken(token);
    wsService.connect();

    // Join chat room after connection is established
    const joinTimer = setTimeout(() => {
      wsService.joinChat(id);
    }, 500);

    // Handle connection restored - retry failed messages
    const unsubscribeConnect = wsService.onConnect(() => {
      console.log('[Chat] WebSocket connected, retrying failed messages');
      retryFailedMessages();
    });

    // Handle connection lost
    const unsubscribeDisconnect = wsService.onDisconnect((reason: string) => {
      console.log('[Chat] WebSocket disconnected:', reason);
      // Mark sending messages as failed when connection is lost
      setMessages(prev => prev.map(m => 
        m.status === 'sending' ? { ...m, status: 'failed' } : m
      ));
    });

    const unsubscribe = wsService.onMessage((data: any) => {
      console.log('[Chat] WS message:', data.type, 'chat:', data.chat_id, 'current:', id);
      
      // Handle messages_loaded event (for current chat)
      if (data.type === 'messages_loaded' && data.chat_id === id) {
        console.log('[Chat] Messages loaded via WebSocket:', data.payload.messages?.length);
        if (data.payload.messages) {
          setMessages(data.payload.messages);
          console.log('[Chat] WebSocket messages set successfully');
        }
        return;
      }
      
      // Handle chats_loaded event
      if (data.type === 'chats_loaded') {
        console.log('[Chat] Chats loaded via WebSocket:', data.payload.chats?.length);
        // Update chats list if needed
        return;
      }
      
      // Other events should be filtered by chat_id
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
          type: data.payload.type || 'text',
          content: data.payload.content,
          created_at: data.payload.created_at,
          updated_at: data.payload.updated_at || data.payload.created_at,
          reply_to_content: data.payload.reply_to_content,
          reply_to_sender_name: data.payload.reply_to_sender_name,
        };
        setMessages(prev => [...prev, newMessage]);
        // Auto-scroll to bottom when new message arrives
        setTimeout(() => {
          listRef.current?.scrollToEnd({ animated: true });
        }, 100);
        // Remove typing indicator when message arrives
        setTypingUsers(prev => {
          const next = new Set(prev);
          next.delete(newMessage.sender_id);
          return next;
        });
      } else if (data.type === 'reaction_added') {
        const { message_id, user_id, emoji, user } = data.payload;
        setMessages(prev => prev.map(m => {
          if (m.id === message_id) {
            const existingReaction = m.reactions?.find(r => r.user_id === user_id && r.emoji === emoji);
            if (!existingReaction) {
              return {
                ...m,
                reactions: [...(m.reactions || []), { id: `${message_id}-${user_id}-${emoji}`, message_id, user_id, emoji, user }]
              };
            }
          }
          return m;
        }));
      } else if (data.type === 'reaction_removed') {
        const { message_id, user_id, emoji } = data.payload;
        setMessages(prev => prev.map(m => {
          if (m.id === message_id) {
            return {
              ...m,
              reactions: (m.reactions || []).filter(r => !(r.user_id === user_id && r.emoji === emoji))
            };
          }
          return m;
        }));
      } else if (data.type === 'message_pinned') {
        const { message_id } = data.payload;
        setMessages(prev => prev.map(m => 
          m.id === message_id ? { ...m, pinned: true } : m
        ));
      } else if (data.type === 'message_unpinned') {
        const { message_id } = data.payload;
        setMessages(prev => prev.map(m => 
          m.id === message_id ? { ...m, pinned: false } : m
        ));
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
      unsubscribeConnect();
      unsubscribeDisconnect();
      wsService.leaveChat(id);
    };
  }, [token, id, retryFailedMessages]);

  const retryMessage = useCallback(async (msg: Message) => {
    if (!id || !user) return;

    // Remove the failed message from UI
    setMessages(prev => prev.filter(m => m.id !== msg.id));
    setFailedMessages(prev => prev.filter(fm => fm.tempId !== msg.id));

    // Create new optimistic message
    const newTempId = `retry-${Date.now()}-${Math.random()}`;
    const optimistic: Message = {
      id: newTempId,
      chat_id: id,
      sender_id: user.id,
      type: msg.type || 'text',
      content: msg.content,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      reply_to: msg.reply_to,
      status: 'sending',
    };
    setMessages(prev => [...prev, optimistic]);

    try {
      const res = await api.sendMessage(id, msg.content, msg.type || 'text', msg.reply_to);
      if (!res.data) throw new Error('Failed to send message');

      setMessages(prev => prev.map(m =>
        m.id === newTempId ? { ...m, id: res.data!.id, status: 'sent' } : m
      ));
    } catch (e) {
      console.error('Retry message error:', e);
      setMessages(prev => prev.map(m =>
        m.id === newTempId ? { ...m, status: 'failed' } : m
      ));
      setFailedMessages(prev => [...prev, {
        tempId: newTempId,
        content: msg.content,
        type: msg.type === 'file' ? 'file' : 'text',
        replyTo: msg.reply_to,
      }]);
    }
  }, [id, user]);

  const deleteMessage = useCallback(async (messageId: string) => {
    if (!id) return;
    try {
      await api.deleteMessage(id, messageId);
      setMessages(prev => prev.filter(m => m.id !== messageId));
    } catch (e) {
      console.log('No permissions for message actions');
      return;
    }
  }, [id]);

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
    return sender || null;
  };

  // Status checkmark component - shows sending, sent, delivered, read, failed states
  const StatusCheckmarks = ({ msg }: { msg: Message }) => {
    const readByOthers = (msg.read_by || []).filter(id => id !== user?.id);
    
    // Failed status - show retry button
    if (msg.status === 'failed') {
      return (
        <TouchableOpacity
          onPress={() => retryMessage(msg)}
          hitSlop={{ top: 8, bottom: 8, left: 8, right: 8 }}
          style={{ flexDirection: 'row', alignItems: 'center' }}>
          <MaterialIcons name="refresh" size={14} color="#ff4444" />
          <ThemedText style={{ fontSize: 10, color: '#ff4444', marginLeft: 2 }}>Повторить</ThemedText>
        </TouchableOpacity>
      );
    }
    
    // Sending status - show loading indicator
    if (msg.status === 'sending') {
      return <ActivityIndicator size="small" color={textColor + '80'} />;
    }
    
    // Delivered status - single checkmark
    if (msg.status === 'delivered' || (!msg.status && readByOthers.length === 0)) {
      return <MaterialIcons name="check" size={14} color={textColor + '80'} />;
    }
    
    // Read status - double checkmark
    if (msg.status === 'read' || (!msg.status && readByOthers.length > 0)) {
      return (
        <View style={styles.doubleCheck}>
          <MaterialIcons name="check" size={14} color={primaryColor} style={{ position: 'absolute', left: 0 }} />
          <MaterialIcons name="check" size={14} color={primaryColor} style={{ position: 'absolute', left: 4 }} />
        </View>
      );
    }
    
    // Default to sent status
    return <MaterialIcons name="check" size={14} color={textColor + '80'} />;
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
      
      // Check if it's an image file
      const imageExtensions = ['.jpg', '.jpeg', '.png', '.gif', '.webp', '.bmp', '.svg'];
      const isImage = imageExtensions.some(ext => url.toLowerCase().endsWith(ext));
      
      if (isImage) {
        setPreviewImageUrl(url);
        setImagePreviewVisible(true);
        return;
      }
      
      if (Platform.OS === 'web') {
        window.open(url, '_blank');
      } else {
        Linking.openURL(url).catch(() => {
          Alert.alert('Ошибка', 'Не удалось открыть файл');
        });
      }
    };

    const showMenuButton = canDeleteMessage || isOwn;

    const bubbleChildren: React.ReactNode[] = [];
    if (msg.file_url) {
      bubbleChildren.push(
        <TouchableOpacity
          key="file"
          onPress={handleOpenFile}
          style={[styles.fileAttachment, { backgroundColor: isOwn ? 'rgba(255,255,255,0.2)' : 'rgba(0,0,0,0.05)' }]}>
          <MaterialIcons name="insert-drive-file" size={24} color={textColor} />
          <ThemedText style={[styles.fileName, { color: textColor }]} numberOfLines={1}>
            {msg.file_url.split('/').pop() || 'Файл'}
          </ThemedText>
          <MaterialIcons name="open-in-new" size={18} color={textColor} style={{ opacity: 0.7 }} />
        </TouchableOpacity>
      );
    }
    if (msg.forwarded_sender_name) {
      bubbleChildren.push(
        <View key="fwd" style={[styles.forwardedBubble, { backgroundColor: isOwn ? 'rgba(255,255,255,0.15)' : 'rgba(0,0,0,0.05)' }]}>
          <MaterialIcons name="forward" size={16} color={textColor} style={{ opacity: 0.8 }} />
          <ThemedText style={[styles.forwardedBubbleText, { color: textColor }]}>
            {'Переслано от ' + msg.forwarded_sender_name}
          </ThemedText>
        </View>
      );
    }
    if (msg.reply_to_content) {
      const replyText = msg.reply_to_content.length > 50 ? msg.reply_to_content.substring(0, 50) + '...' : msg.reply_to_content;
      bubbleChildren.push(
        <View key="reply" style={[styles.forwardedBubble, { backgroundColor: isOwn ? 'rgba(255,255,255,0.15)' : 'rgba(0,0,0,0.05)' }]}>
          <MaterialIcons name="reply" size={16} color={textColor} style={{ opacity: 0.8 }} />
          <ThemedText style={[styles.forwardedBubbleText, { color: textColor }]}>
            {'Ответ: ' + replyText}
          </ThemedText>
        </View>
      );
    }
    if (msg.content) {
      bubbleChildren.push(
        <MentionText key="content" style={[styles.bubbleText, { color: textColor }]} text={msg.content} membersMap={membersMap} />
      );
    }
    bubbleChildren.push(
      <MessageReactions key="reactions" reactions={msg.reactions || []} compact={false} />
    );
    const timeRowChildren: React.ReactNode[] = [];
    if (msg.pinned) {
      timeRowChildren.push(<MaterialIcons key="pin" name="push-pin" size={14} color={primaryColor} style={{ marginRight: 4 }} />);
    }
    timeRowChildren.push(
      <ThemedText key="time" style={[styles.time, { color: textColor, opacity: 0.7 }]}>
        {dayjs(msg.created_at).format('HH:mm')}
      </ThemedText>
    );
    if (msg.updated_at && msg.updated_at !== msg.created_at) {
      timeRowChildren.push(
        <ThemedText key="edited" style={[styles.editedLabel, { color: textColor, opacity: 0.5 }]}>
          (изменено)
        </ThemedText>
      );
    }
    if (showStatus) {
      const statusChildren: React.ReactNode[] = [];
      if (uploading) {
        statusChildren.push(
          <View key="progress" style={styles.uploadProgressContainer}>
            <ActivityIndicator size="small" color={primaryColor} />
            <ThemedText style={styles.uploadProgressText}>
              {'Загрузка файла... ' + Math.round(uploadProgress) + '%'}
            </ThemedText>
          </View>
        );
      }
      statusChildren.push(<StatusCheckmarks key="status" msg={msg} />);
      timeRowChildren.push(
        <View key="statusWrap" style={styles.statusContainer}>
          {statusChildren}
        </View>
      );
    }
    bubbleChildren.push(
      <View key="timeRow" style={styles.timeRow}>
        {timeRowChildren}
      </View>
    );

    const bubbleContent = (
      <Pressable
        onLongPress={() => handleMessageMenuPress(msg)}
        {...contextMenuProps}
        style={({ pressed }) => [
          styles.bubble,
          isOwn ? [styles.bubbleOwn, { backgroundColor: messageOutgoing }] : [styles.bubbleOther, { backgroundColor: messageIncoming }],
          pressed && { opacity: 0.9 }
        ]}>
        {bubbleChildren}
      </Pressable>
    );

    const menuButton = showMenuButton ? (
      <TouchableOpacity
        onPress={() => {
          if (Platform.OS === 'web') {
            handleMessageMenuPress(msg);
          } else {
            handleMessageMenuPress(msg);
          }
        }}
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
      <View style={{ flexDirection: 'row', alignItems: 'flex-end', flexShrink: 1, maxWidth: '100%' }}>
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
  }, [user, membersMap, isGroup, messageOutgoing, messageIncoming, textColor, iconColor, handleMessageMenuPress, primaryColor, userRole, retryMessage]);

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
          extraData={messages}
          scrollEventThrottle={16}
          removeClippedSubviews={true}
          windowSize={21}
          initialNumToRender={10}
          maxToRenderPerBatch={5}
          onScrollToIndexFailed={(info) => {
            const wait = new Promise(resolve => setTimeout(resolve, 500));
            wait.then(() => {
              listRef.current?.scrollToIndex({ index: info.index, animated: true });
            });
          }}
        />
        {selectedFile && (
          <View style={[styles.selectedFileRow, { backgroundColor: surfaceColor }]}>
            {selectedFilePreview ? (
              <Image 
                source={{ uri: selectedFilePreview }} 
                style={styles.selectedFilePreview}
                resizeMode="cover"
              />
            ) : (
              <MaterialIcons name="insert-drive-file" size={20} color={primaryColor} />
            )}
            <ThemedText style={[styles.selectedFileName, { color: textColor }]} numberOfLines={1}>
              {selectedFile.name}
            </ThemedText>
            <TouchableOpacity 
              onPress={() => { 
                setSelectedFile(null);
                setSelectedFilePreview(null);
                if (Platform.OS === 'web' && selectedFilePreview) {
                  URL.revokeObjectURL(selectedFilePreview);
                }
              }}>
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
        {/* Reply Preview */}
        {replyingTo && (
          <View style={[styles.replyPreview, { backgroundColor: surfaceColor, borderTopColor: borderColor }]}>
            <View style={styles.replyPreviewContent}>
              <MaterialIcons name="reply" size={16} color={iconColor} />
              <ThemedText style={[styles.replyPreviewText, { color: iconColor }]}>
                Ответ на {getSenderInfo(replyingTo.sender_id)?.first_name || 'пользователя'}
              </ThemedText>
            </View>
            <TouchableOpacity onPress={() => setReplyingTo(null)}>
              <MaterialIcons name="close" size={20} color={iconColor} />
            </TouchableOpacity>
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
                    
                    <TouchableOpacity 
                      style={styles.actionMenuItem}
                      onPress={() => {
                        console.log('[Chat] Export button in action menu pressed');
                        setActionMenuVisible(false);
                        handleExportChat();
                      }}>
                      <MaterialIcons name="download" size={20} color={iconColor} />
                      <ThemedText style={styles.actionMenuText}>
                        Экспорт чата
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
        calleeName={isGroup ? (chat?.title || 'Групповой звонок') : (other ? `${other.first_name} ${other.last_name}` : 'Пользователь')}
        callType={callType}
      />

      {/* Message Action Menu Modal */}
      <MessageActionMenu
        visible={messageMenuVisible}
        onClose={closeMessageMenu}
        messageId={selectedMessage?.id || ''}
        chatId={id || ''}
        isOwn={selectedMessage?.sender_id === user?.id || false}
        isGroup={isGroup}
        userRole={userRole}
        onDelete={deleteMessage}
        onEdit={(messageId: string, content: string) => {
          setEditingMessage({ id: messageId, content, sender_id: user?.id || '', chat_id: id || '', type: 'text', created_at: '', updated_at: '' } as Message);
          setEditText(content);
          setEditModalVisible(true);
        }}
        onPin={(messageId: string) => {
          // Update message pinned status locally
          setMessages(prev => prev.map(m =>
            m.id === messageId ? { ...m, pinned: true } : m
          ));
        }}
        onUnpin={(messageId: string) => {
          // Update message pinned status locally
          setMessages(prev => prev.map(m =>
            m.id === messageId ? { ...m, pinned: false } : m
          ));
        }}
        onForward={handleForward}
        onReply={(messageId: string) => {
          const message = messages.find(m => m.id === messageId);
          if (message) {
            setReplyingTo(message);
            setInput('');
          }
        }}
        messageContent={selectedMessage?.content}
        isPinned={selectedMessage?.pinned || false}
      />

      {/* Image Preview Modal */}
      <ImagePreview
        visible={imagePreviewVisible}
        onClose={() => setImagePreviewVisible(false)}
        imageUrl={previewImageUrl}
      />

      {/* Forward Chat Selection Modal */}
      <Modal visible={forwardModalVisible} transparent animationType="slide">
        <View style={styles.forwardOverlay}>
          <View style={[styles.forwardContainer, { backgroundColor: surfaceColor }]}>
            <ThemedText style={styles.forwardTitle}>Переслать сообщение</ThemedText>
            <View style={styles.forwardChatList}>
              {userChats.map((chat) => {
                // For direct chats, show the other user's name
                const isDirect = chat.type === 'direct';
                let chatName = chat.title || chat.name || 'Чат';
                
                if (isDirect && chat.members && chat.members.length > 0) {
                  const otherMember = chat.members.find((m: any) => m.user_id !== user?.id);
                  if (otherMember && otherMember.user) {
                    chatName = `${otherMember.user.first_name} ${otherMember.user.last_name}`;
                  }
                }
                
                return (
                  <TouchableOpacity
                    key={chat.id}
                    style={styles.forwardChatItem}
                    onPress={async () => {
                      try {
                        await api.forwardMessage(chat.id, forwardingMessageId || '');
                        Alert.alert('Успешно', 'Сообщение переслано');
                        setForwardModalVisible(false);
                        setForwardingMessageId(null);
                        // Navigate to the target chat
                        router.push(`/chat/${chat.id}`);
                      } catch (error) {
                        console.error('Failed to forward message:', error);
                        Alert.alert('Ошибка', 'Не удалось переслать сообщение');
                      }
                    }}
                  >
                    <ThemedText style={styles.forwardChatName}>{chatName}</ThemedText>
                  </TouchableOpacity>
                );
              })}
              {userChats.length === 0 && (
                <ThemedText style={styles.forwardEmpty}>Нет других чатов</ThemedText>
              )}
            </View>
            <TouchableOpacity
              style={styles.forwardCancelButton}
              onPress={() => {
                setForwardModalVisible(false);
                setForwardingMessageId(null);
              }}
            >
              <ThemedText style={[styles.forwardCancelText, { color: '#dc3545' }]}>Отмена</ThemedText>
            </TouchableOpacity>
          </View>
        </View>
      </Modal>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  safe: { flex: 1 },
  centered: { flex: 1, justifyContent: 'center', alignItems: 'center' },
  keyboard: { flex: 1 },
  list: { paddingHorizontal: 16, paddingTop: 16 },
  listItem: { marginBottom: 12 },
  dateRow: { justifyContent: 'center', marginVertical: 8 },
  dateLabel: {
    fontSize: 12,
    color: '#999',
    textAlign: 'center',
  },
  bubble: Platform.select({
    web: { maxWidth: '75%', minWidth: 140, paddingHorizontal: 14, paddingVertical: 10, borderRadius: 18 },
    default: { maxWidth: '70%', paddingHorizontal: 14, paddingVertical: 10, borderRadius: 18 },
  }),
  bubbleOwn: {
    alignSelf: 'flex-end',
    borderBottomRightRadius: 4,
  },
  bubbleOther: {
    alignSelf: 'flex-start',
    borderBottomLeftRadius: 4,
  },
  bubbleText: {
    fontSize: 16,
    lineHeight: 22,
    ...Platform.select({
      web: { wordBreak: 'break-word' as any },
    }),
  },
  forwardedIndicator: {
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: 4,
    gap: 4,
  },
  forwardedText: {
    fontSize: 11,
    fontStyle: 'italic',
  },
  forwardedBubble: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: 10,
    paddingVertical: 6,
    borderRadius: 8,
    marginBottom: 6,
    gap: 6,
  },
  forwardedBubbleText: {
    fontSize: 12,
    fontWeight: '500',
  },
  replyPreview: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: 16,
    paddingVertical: 8,
    borderTopWidth: 1,
  },
  replyPreviewContent: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
    flex: 1,
  },
  replyPreviewText: {
    fontSize: 12,
  },
  timeRow: {
    flexDirection: 'row',
    alignItems: 'center',
    marginTop: 4,
    gap: 6,
  },
  time: {
    fontSize: 11,
  },
  editedLabel: {
    fontSize: 11,
    fontStyle: 'italic',
  },
  statusContainer: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  uploadProgressContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
    marginRight: 8,
  },
  uploadProgressText: {
    fontSize: 12,
    opacity: 0.7,
  },
  senderName: { fontSize: 12 },
  messageRow: { flexDirection: 'row', marginBottom: 12, alignItems: 'flex-end' },
  messageRowOwn: { justifyContent: 'flex-end' },
  messageRowOther: { justifyContent: 'flex-start' },
  messageRowOtherGroup: { flexDirection: 'row', marginBottom: 12, alignItems: 'flex-start' },
  avatarColumn: { marginRight: 8 },
  avatarLarge: { width: 40, height: 40, borderRadius: 20 },
  avatarFallbackLarge: { width: 40, height: 40, borderRadius: 20, justifyContent: 'center', alignItems: 'center' },
  messageContentColumn: { flexDirection: 'column', flexShrink: 1, maxWidth: '100%' },
  senderNameAbove: { fontSize: 12, marginBottom: 4 },
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
  headerRight: { flexDirection: 'row', alignItems: 'center', gap: 8, zIndex: 1 },
  headerIconButton: { minWidth: 44, minHeight: 44, paddingHorizontal: 8, paddingVertical: 8, justifyContent: 'center', alignItems: 'center' },
  headerTitle: { fontSize: 16, fontWeight: '600' },
  doubleCheck: { width: 22, height: 14, marginLeft: 2 },
  unreadDivider: { flexDirection: 'row', alignItems: 'center', marginVertical: 16, paddingHorizontal: 16 },
  unreadLine: { flex: 1, height: 1 },
  unreadText: { fontSize: 12, marginHorizontal: 12, fontWeight: '500' },
  selectedFilePreview: { width: 40, height: 40, borderRadius: 8 },
  selectedFileName: { flex: 1, fontSize: 14 },
  removeFileButton: { padding: 8 },
  searchOverlay: { flex: 1, justifyContent: 'center', alignItems: 'center', backgroundColor: 'rgba(0,0,0,0.5)' },
  searchContainer: { width: '90%', maxHeight: '80%', borderRadius: 12, padding: 20 },
  searchInput: { borderWidth: 1, borderRadius: 8, padding: 12, fontSize: 16, marginBottom: 12 },
  searchList: { maxHeight: 400 },
  searchItem: { paddingVertical: 12, borderBottomWidth: 1 },
  searchItemText: { fontSize: 14 },
  searchEmpty: { textAlign: 'center', fontSize: 14, marginTop: 20 },
  forwardOverlay: { flex: 1, justifyContent: 'center', alignItems: 'center', backgroundColor: 'rgba(0,0,0,0.5)' },
  forwardContainer: { width: '90%', maxHeight: '80%', borderRadius: 12, padding: 20 },
  forwardTitle: { fontSize: 18, fontWeight: '600', marginBottom: 16 },
  forwardChatList: { maxHeight: 400 },
  forwardChatItem: { paddingVertical: 12, borderBottomWidth: 1 },
  forwardChatName: { fontSize: 16 },
  forwardEmpty: { textAlign: 'center', fontSize: 14, marginTop: 20 },
  forwardCancelButton: { marginTop: 16, paddingVertical: 12, alignItems: 'center' },
  forwardCancelText: { fontSize: 16 },
  modalOverlay: { flex: 1, justifyContent: 'center', alignItems: 'center', backgroundColor: 'rgba(0,0,0,0.5)' },
  modalContent: { width: '85%', borderRadius: 12, padding: 20 },
  modalTitle: { fontSize: 18, fontWeight: '600', marginBottom: 16 },
  modalInput: { borderWidth: 1, borderRadius: 8, padding: 12, fontSize: 16, minHeight: 80, marginBottom: 16 },
  modalButtons: { flexDirection: 'row', justifyContent: 'flex-end', gap: 12 },
  modalButton: { paddingVertical: 10, paddingHorizontal: 16 },
  modalButtonPrimary: { backgroundColor: '#007AFF', borderRadius: 8 },
  attachButton: { padding: 8, justifyContent: 'center', alignItems: 'center' },
  selectedFileRow: { flexDirection: 'row', alignItems: 'center', paddingHorizontal: 16, paddingVertical: 8, gap: 8 },
  fileAttachment: { flexDirection: 'row', alignItems: 'center', padding: 8, borderRadius: 8, marginBottom: 6, gap: 8 },
  fileName: { flex: 1, fontSize: 14 },
  messageMenuButton: {
    padding: 4,
    justifyContent: 'center',
    alignItems: 'center',
    alignSelf: 'flex-start',
    marginBottom: 4,
  },
  threeDotsButton: {
    padding: 4,
    opacity: 0.7,
  },
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
  searchButton: { padding: 8, marginLeft: 8 },
  searchResultsList: { paddingHorizontal: 16 },
  searchResultItem: { paddingVertical: 12, borderBottomWidth: 1 },
  searchResultHeader: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', marginBottom: 4 },
  searchResultSender: { fontSize: 14, fontWeight: '600' },
  searchResultTime: { fontSize: 12, opacity: 0.7 },
  searchResultText: { fontSize: 15 },
  searchResultsEmpty: { paddingVertical: 48, alignItems: 'center' },
});

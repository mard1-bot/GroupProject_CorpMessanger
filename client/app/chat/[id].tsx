import MaterialIcons from '@expo/vector-icons/MaterialIcons';
import { useNavigation } from '@react-navigation/native';
import {
  addMessage,
  getChatById,
  getMessagesByChatId,
  getOtherParticipant,
} from '@/data/mock';
import { useAuthStore } from '@/store/auth';
import { useLocalSearchParams, useRouter } from 'expo-router';
import dayjs from 'dayjs';
import 'dayjs/locale/ru';
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';

dayjs.locale('ru');

type ListItem =
  | { type: 'date'; key: string; label: string }
  | { type: 'message'; key: string; message: Message };

function formatDateLabel(date: dayjs.Dayjs): string {
  const today = dayjs().startOf('day');
  const msgDay = date.startOf('day');
  if (msgDay.isSame(today)) return 'Сегодня';
  if (msgDay.isSame(today.subtract(1, 'day'))) return 'Вчера';
  return date.format('D MMMM YYYY');
}

function buildListItems(messages: Message[]): ListItem[] {
  const items: ListItem[] = [];
  let lastDate = '';
  for (const msg of messages) {
    const d = dayjs(msg.createdAt);
    const dateKey = d.format('YYYY-MM-DD');
    if (dateKey !== lastDate) {
      lastDate = dateKey;
      items.push({ type: 'date', key: 'date-' + dateKey, label: formatDateLabel(d) });
    }
    items.push({ type: 'message', key: msg.id, message: msg });
  }
  return items;
}
import {
  Alert,
  FlatList,
  KeyboardAvoidingView,
  Platform,
  StyleSheet,
  TextInput,
  TouchableOpacity,
  View,
} from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { useThemeColor } from '@/hooks/use-theme-color';
import type { Message } from '@/types/chat';

export default function ChatScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const navigation = useNavigation();
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const currentUser = useAuthStore((s) => s.currentUser);

  const chat = id ? getChatById(id) : undefined;
  const other = chat && currentUser ? getOtherParticipant(chat, currentUser.id) : undefined;

  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const listRef = useRef<FlatList<ListItem>>(null);

  const primaryColor = useThemeColor({}, 'primary');
  const surfaceColor = useThemeColor({}, 'surface');
  const textColor = useThemeColor({}, 'text');
  const iconColor = useThemeColor({}, 'icon');
  const messageOutgoing = useThemeColor({}, 'messageOutgoing');
  const messageIncoming = useThemeColor({}, 'messageIncoming');
  const borderColor = useThemeColor({}, 'border');

  const onMorePress = useCallback(() => {
    Alert.alert('Меню', undefined, [
      { text: 'Найти', onPress: () => {} },
      { text: 'Отмена', style: 'cancel' },
    ]);
  }, []);

  useLayoutEffect(() => {
    navigation.setOptions({
      title: other?.name ?? 'Чат',
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
  }, [navigation, other?.name, router, primaryColor, onMorePress]);

  useEffect(() => {
    if (id) setMessages(getMessagesByChatId(id));
  }, [id]);

  const listItems = useMemo(() => buildListItems(messages), [messages]);

  useEffect(() => {
    if (listItems.length === 0) return;
    const t = setTimeout(() => listRef.current?.scrollToEnd({ animated: false }), 100);
    return () => clearTimeout(t);
  }, [listItems.length]);

  const sendMessage = useCallback(() => {
    const text = input.trim();
    if (!text || !id || !currentUser) return;
    addMessage(id, currentUser.id, text);
    setMessages(getMessagesByChatId(id));
    setInput('');
  }, [id, currentUser, input]);

  const renderItem = useCallback(
    ({ item }: { item: ListItem }) => {
      if (item.type === 'date') {
        return (
          <View style={styles.dateRow}>
            <ThemedText style={[styles.dateLabel, { color: iconColor }]}>{item.label}</ThemedText>
          </View>
        );
      }
      const msg = item.message;
      const isOwn = msg.senderId === currentUser?.id;
      return (
        <View style={[styles.messageRow, isOwn ? styles.messageRowOwn : styles.messageRowOther]}>
          <View
            style={[
              styles.bubble,
              isOwn ? [styles.bubbleOwn, { backgroundColor: messageOutgoing }] : [styles.bubbleOther, { backgroundColor: messageIncoming }],
            ]}>
            <ThemedText style={[styles.bubbleText, { color: textColor }]}>
              {msg.text}
            </ThemedText>
            <ThemedText style={[styles.time, { color: textColor, opacity: 0.7 }]}>
              {dayjs(msg.createdAt).format('HH:mm')}
            </ThemedText>
          </View>
        </View>
      );
    },
    [currentUser?.id, messageOutgoing, messageIncoming, textColor, iconColor]
  );

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
          contentContainerStyle={[styles.list, { paddingBottom: insets.bottom + 60 }]}
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
  messageRow: { marginBottom: 8 },
  messageRowOwn: { alignItems: 'flex-end' },
  messageRowOther: { alignItems: 'flex-start' },
  bubble: { maxWidth: '80%', paddingHorizontal: 14, paddingVertical: 10, borderRadius: 18 },
  bubbleOwn: { borderBottomRightRadius: 4 },
  bubbleOther: { borderBottomLeftRadius: 4 },
  bubbleText: { fontSize: 16 },
  time: { fontSize: 11, marginTop: 4 },
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
});

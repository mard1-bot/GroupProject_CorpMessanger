import dayjs from 'dayjs';

import type { Chat, Message, User } from '@/types/chat';

function nextId(): string {
  return 'msg-' + Date.now() + '-' + Math.random().toString(36).slice(2, 9);
}

export const mockUsers: User[] = [
  { id: 'user-test', name: 'Test', username: 'test' },
  { id: 'user-1', name: 'Алексей', username: 'alex' },
  { id: 'user-2', name: 'Мария', username: 'maria' },
  { id: 'user-3', name: 'Дмитрий', username: 'dmitry' },
];

export const mockChats: Chat[] = [
  {
    id: 'chat-1',
    participantIds: ['user-test', 'user-1'],
    lastMessage: 'До встречи завтра!',
    updatedAt: dayjs().subtract(1, 'hour').toISOString(),
  },
  {
    id: 'chat-2',
    participantIds: ['user-test', 'user-2'],
    lastMessage: 'Получил, спасибо!',
    updatedAt: dayjs().subtract(2, 'hour').toISOString(),
  },
  {
    id: 'chat-3',
    participantIds: ['user-1', 'user-2'],
    lastMessage: 'До встречи завтра!',
    updatedAt: dayjs().subtract(1, 'hour').toISOString(),
  },
  {
    id: 'chat-4',
    participantIds: ['user-1', 'user-3'],
    lastMessage: 'Отправил отчёт',
    updatedAt: dayjs().subtract(3, 'hour').toISOString(),
  },
  {
    id: 'chat-5',
    participantIds: ['user-2', 'user-3'],
    lastMessage: 'Ок, договорились',
    updatedAt: dayjs().subtract(1, 'day').toISOString(),
  },
];

export const mockMessages: Message[] = [
  { id: 'msg-1', chatId: 'chat-1', senderId: 'user-1', text: 'Привет! Как дела?', createdAt: dayjs().subtract(3, 'day').add(10, 'hour').toISOString() },
  { id: 'msg-2', chatId: 'chat-1', senderId: 'user-test', text: 'Привет! Всё отлично, спасибо', createdAt: dayjs().subtract(3, 'day').add(10, 'hour').add(5, 'minute').toISOString() },
  { id: 'msg-3', chatId: 'chat-1', senderId: 'user-1', text: 'Отлично, тогда до связи', createdAt: dayjs().subtract(3, 'day').add(11, 'hour').toISOString() },
  { id: 'msg-4', chatId: 'chat-1', senderId: 'user-test', text: 'Договорились 👍', createdAt: dayjs().subtract(2, 'day').add(9, 'hour').toISOString() },
  { id: 'msg-5', chatId: 'chat-1', senderId: 'user-1', text: 'Доброе утро! Напомни, во сколько созвон?', createdAt: dayjs().subtract(2, 'day').add(10, 'hour').toISOString() },
  { id: 'msg-6', chatId: 'chat-1', senderId: 'user-test', text: 'Доброе! В 15:00 по МСК', createdAt: dayjs().subtract(2, 'day').add(10, 'hour').add(15, 'minute').toISOString() },
  { id: 'msg-7', chatId: 'chat-1', senderId: 'user-1', text: 'Спасибо!', createdAt: dayjs().subtract(2, 'day').add(10, 'hour').add(20, 'minute').toISOString() },
  { id: 'msg-8', chatId: 'chat-1', senderId: 'user-1', text: 'Вчера созвон прошёл хорошо', createdAt: dayjs().subtract(1, 'day').add(14, 'hour').toISOString() },
  { id: 'msg-9', chatId: 'chat-1', senderId: 'user-test', text: 'Да, согласен. Итоги отправил в общий чат', createdAt: dayjs().subtract(1, 'day').add(14, 'hour').add(10, 'minute').toISOString() },
  { id: 'msg-10', chatId: 'chat-1', senderId: 'user-1', text: 'Супер, посмотрю', createdAt: dayjs().subtract(1, 'day').add(15, 'hour').toISOString() },
  { id: 'msg-11', chatId: 'chat-1', senderId: 'user-1', text: 'Напомни, во сколько созвон сегодня?', createdAt: dayjs().subtract(1, 'hour').add(30, 'minute').toISOString() },
  { id: 'msg-12', chatId: 'chat-1', senderId: 'user-test', text: 'В 15:00 по МСК, как обычно', createdAt: dayjs().subtract(1, 'hour').add(25, 'minute').toISOString() },
  { id: 'msg-13', chatId: 'chat-1', senderId: 'user-1', text: 'До встречи завтра!', createdAt: dayjs().subtract(1, 'hour').toISOString() },
  { id: 'msg-14', chatId: 'chat-2', senderId: 'user-2', text: 'Привет! Как продвигается задача?', createdAt: dayjs().subtract(4, 'day').add(12, 'hour').toISOString() },
  { id: 'msg-15', chatId: 'chat-2', senderId: 'user-test', text: 'Привет! Почти готово, сегодня допилю', createdAt: dayjs().subtract(4, 'day').add(12, 'hour').add(30, 'minute').toISOString() },
  { id: 'msg-16', chatId: 'chat-2', senderId: 'user-2', text: 'Отлично, жду', createdAt: dayjs().subtract(4, 'day').add(13, 'hour').toISOString() },
  { id: 'msg-17', chatId: 'chat-2', senderId: 'user-test', text: 'Привет, отчёт готов', createdAt: dayjs().subtract(4, 'hour').toISOString() },
  { id: 'msg-18', chatId: 'chat-2', senderId: 'user-2', text: 'Супер, скинь ссылку', createdAt: dayjs().subtract(3, 'hour').add(50, 'minute').toISOString() },
  { id: 'msg-19', chatId: 'chat-2', senderId: 'user-test', text: 'Отправил отчёт в общий чат', createdAt: dayjs().subtract(3, 'hour').toISOString() },
  { id: 'msg-20', chatId: 'chat-2', senderId: 'user-2', text: 'Получил, спасибо!', createdAt: dayjs().subtract(2, 'hour').toISOString() },
  { id: 'msg-21', chatId: 'chat-3', senderId: 'user-2', text: 'Перенесём на пятницу?', createdAt: dayjs().subtract(1, 'day').add(2, 'hour').toISOString() },
  { id: 'msg-22', chatId: 'chat-3', senderId: 'user-1', text: 'Ок, договорились', createdAt: dayjs().subtract(1, 'day').toISOString() },
];

export function getMessagesByChatId(chatId: string): Message[] {
  return mockMessages.filter((m) => m.chatId === chatId).sort((a, b) => new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime());
}

export function getChatById(chatId: string): Chat | undefined {
  return mockChats.find((c) => c.id === chatId);
}

export function getUserById(userId: string): User | undefined {
  return mockUsers.find((u) => u.id === userId);
}

export function getOtherParticipant(chat: Chat, currentUserId: string): User | undefined {
  const otherId = chat.participantIds.find((id) => id !== currentUserId);
  return otherId ? getUserById(otherId) : undefined;
}

export function addMessage(chatId: string, senderId: string, text: string): Message {
  const message: Message = {
    id: nextId(),
    chatId,
    senderId,
    text,
    createdAt: new Date().toISOString(),
  };
  mockMessages.push(message);
  const chat = mockChats.find((c) => c.id === chatId);
  if (chat) {
    chat.lastMessage = text;
    chat.updatedAt = message.createdAt;
  }
  return message;
}

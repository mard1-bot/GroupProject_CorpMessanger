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

const t = (d: dayjs.Dayjs) => d.toISOString();

export const mockChats: Chat[] = [
  {
    id: 'chat-1',
    participantIds: ['user-test', 'user-1'],
    lastMessage: 'Ок, завтра до 11 кину в общий',
    updatedAt: t(dayjs().subtract(42, 'minute')),
  },
  {
    id: 'chat-2',
    participantIds: ['user-test', 'user-2'],
    lastMessage: 'Обнимаю в ответ 😄',
    updatedAt: t(dayjs().subtract(2, 'hour').subtract(6, 'minute')),
  },
  {
    id: 'chat-3',
    participantIds: ['user-1', 'user-2'],
    lastMessage: 'Тогда в 12:00 в переговорке «Берёза»',
    updatedAt: t(dayjs().subtract(5, 'hour').subtract(18, 'minute')),
  },
  {
    id: 'chat-4',
    participantIds: ['user-1', 'user-3'],
    lastMessage: 'Закинул в ветку deploy/fix-staging',
    updatedAt: t(dayjs().subtract(3, 'hour').subtract(4, 'minute')),
  },
  {
    id: 'chat-5',
    participantIds: ['user-2', 'user-3'],
    lastMessage: 'Ок, договорились',
    updatedAt: t(dayjs().subtract(1, 'day').subtract(3, 'hour')),
  },
];

export const mockMessages: Message[] = [
  {
    id: 'msg-c1-01',
    chatId: 'chat-1',
    senderId: 'user-1',
    text: 'Ты успел глянуть PR с логином? Вчера допушил фикс по refresh',
    createdAt: t(dayjs().subtract(6, 'day').hour(11).minute(7).second(12)),
  },
  {
    id: 'msg-c1-02',
    chatId: 'chat-1',
    senderId: 'user-test',
    text: 'привет, пока только дифф пробежалась глазами',
    createdAt: t(dayjs().subtract(6, 'day').hour(11).minute(19).second(44)),
  },
  {
    id: 'msg-c1-03',
    chatId: 'chat-1',
    senderId: 'user-test',
    text: 'до рантайма дотянусь сегодня после обеда, отпишусь',
    createdAt: t(dayjs().subtract(6, 'day').hour(11).minute(20).second(3)),
  },
  {
    id: 'msg-c1-04',
    chatId: 'chat-1',
    senderId: 'user-1',
    text: 'ок ок, без паники 🙂 главное до кандидата на релиз',
    createdAt: t(dayjs().subtract(6, 'day').hour(11).minute(38).second(51)),
  },
  {
    id: 'msg-c1-05',
    chatId: 'chat-1',
    senderId: 'user-test',
    text: 'ага поняла',
    createdAt: t(dayjs().subtract(6, 'day').hour(11).minute(39).second(10)),
  },
  {
    id: 'msg-c1-06',
    chatId: 'chat-1',
    senderId: 'user-test',
    text: 'глянула — в целом норм, один edge case в middleware, написала в тред',
    createdAt: t(dayjs().subtract(5, 'day').hour(16).minute(22).second(0)),
  },
  {
    id: 'msg-c1-07',
    chatId: 'chat-1',
    senderId: 'user-1',
    text: 'увидел, спасибо! сегодня вечером залью',
    createdAt: t(dayjs().subtract(5, 'day').hour(17).minute(5).second(33)),
  },
  {
    id: 'msg-c1-08',
    chatId: 'chat-1',
    senderId: 'user-1',
    text: 'залил, глянешь когда минутка?',
    createdAt: t(dayjs().subtract(4, 'day').hour(19).minute(41).second(8)),
  },
  {
    id: 'msg-c1-09',
    chatId: 'chat-1',
    senderId: 'user-test',
    text: 'сек',
    createdAt: t(dayjs().subtract(4, 'day').hour(19).minute(48).second(2)),
  },
  {
    id: 'msg-c1-10',
    chatId: 'chat-1',
    senderId: 'user-test',
    text: 'всё чисто, можно мержить 👍',
    createdAt: t(dayjs().subtract(4, 'day').hour(19).minute(52).second(19)),
  },
  {
    id: 'msg-c1-11',
    chatId: 'chat-1',
    senderId: 'user-1',
    text: 'красава',
    createdAt: t(dayjs().subtract(4, 'day').hour(19).minute(53).second(40)),
  },
  {
    id: 'msg-c1-12',
    chatId: 'chat-1',
    senderId: 'user-1',
    text: 'кстати вчера созвон норм прошёл, только Дима опять на 7 мин опоздал',
    createdAt: t(dayjs().subtract(2, 'day').hour(10).minute(14).second(0)),
  },
  {
    id: 'msg-c1-13',
    chatId: 'chat-1',
    senderId: 'user-test',
    text: 'классика 😅 итоги я в общий закинула',
    createdAt: t(dayjs().subtract(2, 'day').hour(10).minute(21).second(55)),
  },
  {
    id: 'msg-c1-14',
    chatId: 'chat-1',
    senderId: 'user-1',
    text: 'супер, гляну вечером',
    createdAt: t(dayjs().subtract(2, 'day').hour(10).minute(23).second(12)),
  },
  {
    id: 'msg-c1-15',
    chatId: 'chat-1',
    senderId: 'user-1',
    text: 'напомни плз, созвон сегодня во сколько? голова кипятком',
    createdAt: t(dayjs().subtract(1, 'day').hour(18).minute(33).second(0)),
  },
  {
    id: 'msg-c1-16',
    chatId: 'chat-1',
    senderId: 'user-test',
    text: '15:00 мск как всегда',
    createdAt: t(dayjs().subtract(1, 'day').hour(18).minute(35).second(28)),
  },
  {
    id: 'msg-c1-17',
    chatId: 'chat-1',
    senderId: 'user-1',
    text: 'спасибо ты лучшая',
    createdAt: t(dayjs().subtract(1, 'day').hour(18).minute(36).second(1)),
  },
  {
    id: 'msg-c1-18',
    chatId: 'chat-1',
    senderId: 'user-1',
    text: 'и завтра демо клиенту — кинь черновик слайдов в общий до 11?',
    createdAt: t(dayjs().subtract(1, 'hour').subtract(51, 'minute')),
  },
  {
    id: 'msg-c1-19',
    chatId: 'chat-1',
    senderId: 'user-test',
    text: 'Ок, завтра до 11 кину в общий',
    createdAt: t(dayjs().subtract(42, 'minute')),
  },

  {
    id: 'msg-c2-01',
    chatId: 'chat-2',
    senderId: 'user-2',
    text: 'привет) как по задаче с отчётом? не горит, просто спрашиваю',
    createdAt: t(dayjs().subtract(5, 'day').hour(14).minute(2).second(0)),
  },
  {
    id: 'msg-c2-02',
    chatId: 'chat-2',
    senderId: 'user-test',
    text: 'привет! почти — остались графики и вывод по выручке',
    createdAt: t(dayjs().subtract(5, 'day').hour(14).minute(18).second(40)),
  },
  {
    id: 'msg-c2-03',
    chatId: 'chat-2',
    senderId: 'user-2',
    text: 'ооо',
    createdAt: t(dayjs().subtract(5, 'day').hour(14).minute(19).second(5)),
  },
  {
    id: 'msg-c2-04',
    chatId: 'chat-2',
    senderId: 'user-2',
    text: 'ориентир по времени есть? не давлю, правда',
    createdAt: t(dayjs().subtract(5, 'day').hour(14).minute(19).second(48)),
  },
  {
    id: 'msg-c2-05',
    chatId: 'chat-2',
    senderId: 'user-test',
    text: 'к вечеру закрою, не позже 19',
    createdAt: t(dayjs().subtract(5, 'day').hour(14).minute(25).second(11)),
  },
  {
    id: 'msg-c2-06',
    chatId: 'chat-2',
    senderId: 'user-2',
    text: 'супер, тогда жду ❤️ (шучу, просто жду)',
    createdAt: t(dayjs().subtract(5, 'day').hour(14).minute(26).second(30)),
  },
  {
    id: 'msg-c2-07',
    chatId: 'chat-2',
    senderId: 'user-test',
    text: 'лол спасибо за здоровое давление ))',
    createdAt: t(dayjs().subtract(5, 'day').hour(14).minute(27).second(55)),
  },
  {
    id: 'msg-c2-08',
    chatId: 'chat-2',
    senderId: 'user-2',
    text: 'слушай а если что не успеешь — скажи заранее, переставим приоритет',
    createdAt: t(dayjs().subtract(4, 'day').hour(9).minute(40).second(0)),
  },
  {
    id: 'msg-c2-09',
    chatId: 'chat-2',
    senderId: 'user-test',
    text: 'должна успеть, если что отпишусь до 17',
    createdAt: t(dayjs().subtract(4, 'day').hour(9).minute(52).second(18)),
  },
  {
    id: 'msg-c2-10',
    chatId: 'chat-2',
    senderId: 'user-test',
    text: 'кинула pdf в общий + ссылку на таблицу',
    createdAt: t(dayjs().subtract(3, 'hour').subtract(22, 'minute')),
  },
  {
    id: 'msg-c2-11',
    chatId: 'chat-2',
    senderId: 'user-2',
    text: 'вижу, листаю',
    createdAt: t(dayjs().subtract(3, 'hour').subtract(14, 'minute')),
  },
  {
    id: 'msg-c2-12',
    chatId: 'chat-2',
    senderId: 'user-2',
    text: 'всё ок, цифры сходятся. спасибо огромное 🙏',
    createdAt: t(dayjs().subtract(2, 'hour').subtract(9, 'minute')),
  },
  {
    id: 'msg-c2-13',
    chatId: 'chat-2',
    senderId: 'user-test',
    text: 'Обнимаю в ответ 😄',
    createdAt: t(dayjs().subtract(2, 'hour').subtract(6, 'minute')),
  },

  {
    id: 'msg-c3-01',
    chatId: 'chat-3',
    senderId: 'user-2',
    text: 'слушай, а мы можем синк по интеграции не в чт а в пт?',
    createdAt: t(dayjs().subtract(2, 'day').hour(11).minute(5).second(0)),
  },
  {
    id: 'msg-c3-02',
    chatId: 'chat-3',
    senderId: 'user-1',
    text: 'в пт у меня до обеда только окно',
    createdAt: t(dayjs().subtract(2, 'day').hour(11).minute(48).second(22)),
  },
  {
    id: 'msg-c3-03',
    chatId: 'chat-3',
    senderId: 'user-2',
    text: 'тогда чт в 16:00? или тебе поздно?',
    createdAt: t(dayjs().subtract(2, 'day').hour(12).minute(1).second(0)),
  },
  {
    id: 'msg-c3-04',
    chatId: 'chat-3',
    senderId: 'user-1',
    text: 'чт норм',
    createdAt: t(dayjs().subtract(2, 'day').hour(12).minute(3).second(15)),
  },
  {
    id: 'msg-c3-05',
    chatId: 'chat-3',
    senderId: 'user-2',
    text: 'перенесём на пятницу всё-таки — в чт вылезает релиз',
    createdAt: t(dayjs().subtract(1, 'day').hour(10).minute(20).second(0)),
  },
  {
    id: 'msg-c3-06',
    chatId: 'chat-3',
    senderId: 'user-1',
    text: 'блин ну ладно',
    createdAt: t(dayjs().subtract(1, 'day').hour(10).minute(45).second(30)),
  },
  {
    id: 'msg-c3-07',
    chatId: 'chat-3',
    senderId: 'user-1',
    text: 'пятница 12:00 ок?',
    createdAt: t(dayjs().subtract(1, 'day').hour(10).minute(46).second(0)),
  },
  {
    id: 'msg-c3-08',
    chatId: 'chat-3',
    senderId: 'user-2',
    text: 'Тогда в 12:00 в переговорке «Берёза»',
    createdAt: t(dayjs().subtract(5, 'hour').subtract(18, 'minute')),
  },

  {
    id: 'msg-c4-01',
    chatId: 'chat-4',
    senderId: 'user-3',
    text: 'Алексей, staging снова 502 на /health',
    createdAt: t(dayjs().subtract(2, 'day').hour(15).minute(10).second(0)),
  },
  {
    id: 'msg-c4-02',
    chatId: 'chat-4',
    senderId: 'user-1',
    text: 'вижу в алертах, ща',
    createdAt: t(dayjs().subtract(2, 'day').hour(15).minute(14).second(33)),
  },
  {
    id: 'msg-c4-03',
    chatId: 'chat-4',
    senderId: 'user-1',
    text: 'там nginx крутил вчера, откатил конфиг — должно ожить',
    createdAt: t(dayjs().subtract(2, 'day').hour(15).minute(41).second(0)),
  },
  {
    id: 'msg-c4-04',
    chatId: 'chat-4',
    senderId: 'user-3',
    text: 'да, поднялось. спасибо',
    createdAt: t(dayjs().subtract(2, 'day').hour(16).minute(2).second(18)),
  },
  {
    id: 'msg-c4-05',
    chatId: 'chat-4',
    senderId: 'user-3',
    text: 'на всякий случай: залил фикс лимитов на бэке, глянь когда сможешь',
    createdAt: t(dayjs().subtract(3, 'hour').subtract(11, 'minute')),
  },
  {
    id: 'msg-c4-06',
    chatId: 'chat-4',
    senderId: 'user-1',
    text: 'Закинул в ветку deploy/fix-staging',
    createdAt: t(dayjs().subtract(3, 'hour').subtract(4, 'minute')),
  },

  {
    id: 'msg-c5-01',
    chatId: 'chat-5',
    senderId: 'user-2',
    text: 'Дим, ты можешь завтра подменить меня на груминге?',
    createdAt: t(dayjs().subtract(3, 'day').hour(16).minute(0).second(0)),
  },
  {
    id: 'msg-c5-02',
    chatId: 'chat-5',
    senderId: 'user-3',
    text: 'если до 11:30 — да',
    createdAt: t(dayjs().subtract(3, 'day').hour(16).minute(22).second(0)),
  },
  {
    id: 'msg-c5-03',
    chatId: 'chat-5',
    senderId: 'user-2',
    text: 'там в 11:00 как раз',
    createdAt: t(dayjs().subtract(3, 'day').hour(16).minute(24).second(40)),
  },
  {
    id: 'msg-c5-04',
    chatId: 'chat-5',
    senderId: 'user-3',
    text: 'ок, скидывай повестку вечером',
    createdAt: t(dayjs().subtract(3, 'day').hour(16).minute(30).second(5)),
  },
  {
    id: 'msg-c5-05',
    chatId: 'chat-5',
    senderId: 'user-2',
    text: 'Ок, договорились',
    createdAt: t(dayjs().subtract(1, 'day').subtract(3, 'hour')),
  },
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

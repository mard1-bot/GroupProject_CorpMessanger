export type User = {
  id: string;
  name: string;
  username: string;
  avatarUrl?: string;
};

export type Chat = {
  id: string;
  participantIds: string[];
  lastMessage?: string;
  updatedAt: string;
};

export type Message = {
  id: string;
  chatId: string;
  senderId: string;
  text: string;
  createdAt: string;
};

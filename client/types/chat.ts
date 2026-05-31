export type User = {
  id: string;
  email: string;
  name: string;
  username: string;
  first_name: string;
  last_name: string;
  avatarUrl?: string;
};

export type Chat = {
  id: string;
  type: string;
  title?: string;
  description?: string;
  avatar?: string;
  creator_id: string;
  created_at: string;
  updated_at: string;
  last_message?: string;
  last_message_at?: string;
  members?: {
    user_id: string;
    role?: string;
    muted?: boolean;
    pinned?: boolean;
    user?: {
      id: string;
      email: string;
      name: string;
      username: string;
      first_name: string;
      last_name: string;
      avatar?: string;
    };
  }[];
};

export type Message = {
  id: string;
  chat_id: string;
  sender_id: string;
  type: string;
  content: string;
  file_url?: string;
  created_at: string;
  updated_at?: string;
  read_by?: string[];
  reactions?: Reaction[];
  pinned?: boolean;
  scheduled_at?: string;
  thread_id?: string;
  reply_to?: string;
  reply_to_content?: string;
  reply_to_sender_name?: string;
  forwarded_from?: string;
  forwarded_sender_name?: string;
  status?: 'sending' | 'sent' | 'delivered' | 'read' | 'failed';
};

export type Reaction = {
  id: string;
  message_id: string;
  user_id: string;
  emoji: string;
  user?: {
    id: string;
    first_name: string;
    last_name: string;
  };
};

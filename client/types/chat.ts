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
  members?: {
    user_id: string;
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
  created_at: string;
  updated_at?: string;
  read_by?: string[];
};

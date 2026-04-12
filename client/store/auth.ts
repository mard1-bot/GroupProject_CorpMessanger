import { create } from 'zustand';

import { mockUsers } from '@/data/mock';
import type { User } from '@/types/chat';

type AuthState = {
  currentUser: User | null;
  login: (username: string, password: string) => boolean;
  register: (name: string, username: string, password: string) => boolean;
  logout: () => void;
};

export const useAuthStore = create<AuthState>((set) => ({
  currentUser: null,

  login: (username: string, password: string) => {
    if (!username.trim() || !password.trim()) return false;
    const user = mockUsers.find((u) => u.username.toLowerCase() === username.trim().toLowerCase());
    if (!user) return false;
    set({ currentUser: user });
    return true;
  },

  register: (name: string, username: string, password: string) => {
    if (!name.trim() || !username.trim() || !password.trim()) return false;
    const exists = mockUsers.some((u) => u.username.toLowerCase() === username.trim().toLowerCase());
    if (exists) return false;
    const newUser: User = {
      id: `user-${Date.now()}`,
      name: name.trim(),
      username: username.trim().toLowerCase(),
    };
    mockUsers.push(newUser);
    set({ currentUser: newUser });
    return true;
  },

  logout: () => set({ currentUser: null }),
}));

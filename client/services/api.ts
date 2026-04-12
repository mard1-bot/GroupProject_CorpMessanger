import { Platform } from 'react-native';

// Configuration: Update this to match your backend setup
// For physical device testing, use your computer's local IP address
// You can find it with: ipconfig (Windows) or ifconfig (Mac/Linux)
const COMPUTER_IP = process.env.EXPO_PUBLIC_BACKEND_IP || '192.168.0.103';
const BACKEND_PORT = process.env.EXPO_PUBLIC_BACKEND_PORT || '3000';
const USE_LOCALHOST = process.env.EXPO_PUBLIC_USE_LOCALHOST === 'true';

// API URL - automatically selects the correct URL for each platform
const API_URL = Platform.select({
  ios: USE_LOCALHOST 
    ? `http://localhost:${BACKEND_PORT}` 
    : `http://${COMPUTER_IP}:${BACKEND_PORT}`,     // iOS Simulator
  android: `http://${COMPUTER_IP}:${BACKEND_PORT}`, // Android (physical device) - use IP
  default: `http://localhost:${BACKEND_PORT}`, // Web / fallback
});

// For Android Emulator use: 'http://10.0.2.2:8080'
// For production, set your deployed backend URL
// Configure via environment variables:
// EXPO_PUBLIC_BACKEND_IP=your.ip.address
// EXPO_PUBLIC_BACKEND_PORT=3000

export interface ApiError {
  code: string;
  message: string;
}

export interface ApiResponse<T> {
  data?: T;
  error?: ApiError;
}

export interface User {
  id: string;
  email: string;
  phone?: string;
  first_name: string;
  last_name: string;
  middle_name?: string;
  avatar?: string;
  status: string;
  role: string;
  created_at: string;
  updated_at: string;
}

export interface Chat {
  id: string;
  type: string;
  title?: string;
  description?: string;
  avatar?: string;
  creator_id: string;
  created_at: string;
  updated_at: string;
}

export interface ChatMember {
  chat_id: string;
  user_id: string;
  role: string;
  joined_at: string;
  last_read_at: string;
}

export interface Message {
  id: string;
  chat_id: string;
  sender_id: string;
  type: string;
  content: string;
  created_at: string;
  updated_at: string;
  reply_to?: string;
}

export interface AuthResponse {
  token: string;
  user: User;
}

class ApiClient {
  private baseUrl: string;
  private token: string | null = null;

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl.replace(/\/$/, '');
  }

  setToken(token: string | null) {
    this.token = token;
  }

  getToken(): string | null {
    return this.token;
  }

  private async request<T>(
    method: string,
    path: string,
    body?: object
  ): Promise<ApiResponse<T>> {
    const url = `${this.baseUrl}${path}`;
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };

    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    }

    try {
      const response = await fetch(url, {
        method,
        headers,
        body: body ? JSON.stringify(body) : undefined,
      });

      const contentType = response.headers.get('content-type');
      const isJson = contentType?.includes('application/json');

      if (!response.ok) {
        if (isJson) {
          const errorData = await response.json();
          return {
            error: errorData.error || {
              code: `http_${response.status}`,
              message: response.statusText,
            },
          };
        }
        return {
          error: {
            code: `http_${response.status}`,
            message: response.statusText || 'Unknown error',
          },
        };
      }

      if (response.status === 204) {
        return { data: undefined as T };
      }

      if (isJson) {
        const data = await response.json();
        return { data };
      }

      return { data: undefined as T };
    } catch (error) {
      return {
        error: {
          code: 'network_error',
          message: error instanceof Error ? error.message : 'Network error',
        },
      };
    }
  }

  // Health check
  async health(): Promise<ApiResponse<{ status: string }>> {
    return this.request('GET', '/health');
  }

  // Auth
  async register(data: {
    email: string;
    password: string;
    first_name: string;
    last_name: string;
    middle_name?: string;
    phone?: string;
  }): Promise<ApiResponse<AuthResponse>> {
    return this.request('POST', '/api/v1/auth/register', data);
  }

  async login(data: {
    email: string;
    password: string;
  }): Promise<ApiResponse<AuthResponse>> {
    return this.request('POST', '/api/v1/auth/login', data);
  }

  async logout(): Promise<ApiResponse<{ message: string }>> {
    return this.request('POST', '/api/v1/auth/logout');
  }

  async getCurrentUser(): Promise<ApiResponse<User>> {
    return this.request('GET', '/api/v1/auth/me');
  }

  // Users
  async getUsers(): Promise<ApiResponse<User[]>> {
    return this.request('GET', '/api/v1/users');
  }

  async getUserById(id: string): Promise<ApiResponse<User>> {
    return this.request('GET', `/api/v1/users/${id}`);
  }

  async updateCurrentUser(data: Partial<User>): Promise<ApiResponse<User>> {
    return this.request('PUT', '/api/v1/users/me', data);
  }

  // Chats
  async createChat(data: {
    type: string;
    title?: string;
    description?: string;
    member_ids?: string[];
  }): Promise<ApiResponse<Chat & { members: ChatMember[] }>> {
    return this.request('POST', '/api/v1/chats', data);
  }

  async getUserChats(): Promise<ApiResponse<Chat[]>> {
    return this.request('GET', '/api/v1/chats');
  }

  async getChatById(id: string): Promise<ApiResponse<Chat & { members: ChatMember[] }>> {
    return this.request('GET', `/api/v1/chats/${id}`);
  }

  async addChatMember(chatId: string, userId: string, role?: string): Promise<ApiResponse<ChatMember>> {
    return this.request('POST', `/api/v1/chats/${chatId}/members`, {
      user_id: userId,
      role,
    });
  }

  // Messages
  async sendMessage(
    chatId: string,
    content: string,
    type?: string,
    replyTo?: string
  ): Promise<ApiResponse<Message>> {
    return this.request('POST', `/api/v1/chats/${chatId}/messages`, {
      content,
      type,
      reply_to: replyTo,
    });
  }

  async getChatMessages(
    chatId: string,
    limit?: number,
    offset?: number
  ): Promise<ApiResponse<Message[]>> {
    const params = new URLSearchParams();
    if (limit !== undefined) params.append('limit', String(limit));
    if (offset !== undefined) params.append('offset', String(offset));
    const query = params.toString() ? `?${params.toString()}` : '';
    return this.request('GET', `/api/v1/chats/${chatId}/messages${query}`);
  }
}

export const api = new ApiClient(API_URL);
export default api;

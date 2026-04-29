import { API_URL } from './api';

// WebSocket event types
export const WS_EVENTS = {
  NEW_MESSAGE: 'new_message',
  MESSAGE_UPDATED: 'message_updated',
  MESSAGE_DELETED: 'message_deleted',
  TYPING: 'typing',
  READ_RECEIPT: 'read_receipt',
  PRESENCE: 'presence',
  USER_JOINED: 'user_joined',
  USER_LEFT: 'user_left',
  CHAT_CREATED: 'chat_created',
  CHAT_DELETED: 'chat_deleted',
  CHAT_UPDATED: 'chat_updated',
  ERROR: 'error',
  // WebRTC call events
  CALL_OFFER: 'call_offer',
  CALL_ANSWER: 'call_answer',
  CALL_ICE: 'call_ice',
  CALL_END: 'call_end',
  CALL_REJECT: 'call_reject',
  CALL_ACCEPT: 'call_accept',
  CALL_BUSY: 'call_busy',
  CALL_RINGING: 'call_ringing',
} as const;

export type WebSocketEvent = (typeof WS_EVENTS)[keyof typeof WS_EVENTS];

// Message payload types
export interface WSMessagePayload {
  id: string;
  chat_id: string;
  sender_id: string;
  type: string;
  content: string;
  created_at: string;
  reply_to?: string;
}

export interface WSTypingPayload {
  chat_id: string;
  user_id: string;
  first_name?: string;
  last_name?: string;
  is_typing: boolean;
}

export interface WSReadReceiptPayload {
  chat_id: string;
  user_id: string;
  message_id: string;
  read_at: string;
}

export interface WSPresencePayload {
  user_id: string;
  status: 'online' | 'offline';
  last_seen?: string;
}

export interface WSMessage {
  type: WebSocketEvent;
  chat_id: string;
  payload: WSMessagePayload | WSTypingPayload | WSReadReceiptPayload | WSPresencePayload | any;
}

// Callback types
type MessageCallback = (message: WSMessage) => void;
type ConnectCallback = () => void;
type DisconnectCallback = (reason: string) => void;
type ErrorCallback = (error: Event) => void;

class WebSocketService {
  private ws: WebSocket | null = null;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  private reconnectDelay = 1000; // Start with 1s, exponential backoff
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private heartbeatInterval: ReturnType<typeof setInterval> | null = null;
  private isConnecting = false;

  // Event handlers
  private onMessageCallbacks: MessageCallback[] = [];
  private onConnectCallbacks: ConnectCallback[] = [];
  private onDisconnectCallbacks: DisconnectCallback[] = [];
  private onErrorCallbacks: ErrorCallback[] = [];

  private token: string | null = null;

  // Message queue for messages sent while disconnected
  private messageQueue: Array<{ type: string; payload: any }> = [];

  // Set auth token for WebSocket connection
  setToken(token: string): void {
    this.token = token;
  }

  // Get WebSocket URL from API URL
  private getWSUrl(): string {
    const wsProtocol = API_URL.startsWith('https') ? 'wss' : 'ws';
    const baseUrl = API_URL.replace(/^https?:\/\//, '');
    if (!this.token) {
      throw new Error('WebSocket token not set. Call setToken() before connect()');
    }
    return `${wsProtocol}://${baseUrl}/ws?token=${this.token}`;
  }

  // Connect to WebSocket
  connect(): void {
    if (this.ws?.readyState === WebSocket.OPEN || this.isConnecting) {
      return;
    }

    this.isConnecting = true;

    let wsCreated = false;
    try {
      const wsUrl = this.getWSUrl();
      this.ws = new WebSocket(wsUrl);
      wsCreated = true;

      this.ws.onopen = () => {
        this.isConnecting = false;
        this.reconnectAttempts = 0;
        this.reconnectDelay = 1000;
        this.startHeartbeat();
        
        // Send queued messages
        this.flushMessageQueue();
        
        this.onConnectCallbacks.forEach((cb) => cb());
        console.log('[WebSocket] Connected');
      };

      this.ws.onmessage = (event) => {
        try {
          const message: WSMessage = JSON.parse(event.data);
          this.onMessageCallbacks.forEach((cb) => cb(message));
        } catch (err) {
          console.error('[WebSocket] Failed to parse message:', err);
        }
      };

      this.ws.onclose = (event) => {
        this.isConnecting = false;
        this.stopHeartbeat();
        this.ws = null;

        const reason = event.wasClean ? 'closed' : 'disconnected';
        this.onDisconnectCallbacks.forEach((cb) => cb(reason));
        console.log(`[WebSocket] Disconnected: ${reason}`);

        // Attempt reconnection if not clean close
        if (!event.wasClean && this.reconnectAttempts < this.maxReconnectAttempts) {
          this.scheduleReconnect();
        }
      };

      this.ws.onerror = (error) => {
        this.onErrorCallbacks.forEach((cb) => cb(error));
        console.error('[WebSocket] Error:', error);
      };
    } catch (err) {
      console.error('[WebSocket] Connection failed:', err);
      this.scheduleReconnect();
    } finally {
      // Always reset isConnecting if WebSocket wasn't created
      // or if an error occurred during setup
      if (!wsCreated) {
        this.isConnecting = false;
      }
    }
  }

  // Disconnect from WebSocket
  disconnect(): void {
    this.clearReconnectTimer();
    this.stopHeartbeat();

    if (this.ws) {
      this.ws.close(1000, 'Client disconnect');
      this.ws = null;
    }
  }

  // Send message to server
  send(type: string, payload: any): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ type, payload }));
    } else {
      console.warn('[WebSocket] Cannot send, not connected - queuing message');
      this.messageQueue.push({ type, payload });
    }
  }

  // Join a chat room
  joinChat(chatId: string): void {
    this.send('join_chat', { chat_id: chatId });
  }

  // Leave a chat room
  leaveChat(chatId: string): void {
    this.send('leave_chat', { chat_id: chatId });
  }

  // Send typing indicator
  sendTyping(chatId: string, isTyping: boolean, firstName?: string, lastName?: string): void {
    this.send('typing', {
      chat_id: chatId,
      is_typing: isTyping,
      first_name: firstName,
      last_name: lastName,
    });
  }

  // Send read receipt
  sendReadReceipt(chatId: string, messageId: string): void {
    this.send('read_receipt', { chat_id: chatId, message_id: messageId });
  }

  // Event listeners
  onMessage(callback: MessageCallback): () => void {
    this.onMessageCallbacks.push(callback);
    return () => {
      const index = this.onMessageCallbacks.indexOf(callback);
      if (index > -1) this.onMessageCallbacks.splice(index, 1);
    };
  }

  onConnect(callback: ConnectCallback): () => void {
    this.onConnectCallbacks.push(callback);
    return () => {
      const index = this.onConnectCallbacks.indexOf(callback);
      if (index > -1) this.onConnectCallbacks.splice(index, 1);
    };
  }

  onDisconnect(callback: DisconnectCallback): () => void {
    this.onDisconnectCallbacks.push(callback);
    return () => {
      const index = this.onDisconnectCallbacks.indexOf(callback);
      if (index > -1) this.onDisconnectCallbacks.splice(index, 1);
    };
  }

  onError(callback: ErrorCallback): () => void {
    this.onErrorCallbacks.push(callback);
    return () => {
      const index = this.onErrorCallbacks.indexOf(callback);
      if (index > -1) this.onErrorCallbacks.splice(index, 1);
    };
  }

  // Check connection status
  isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }

  // Private methods
  private scheduleReconnect(): void {
    this.clearReconnectTimer();
    this.reconnectAttempts++;

    console.log(`[WebSocket] Reconnecting in ${this.reconnectDelay}ms (attempt ${this.reconnectAttempts})`);

    this.reconnectTimer = setTimeout(() => {
      this.connect();
    }, this.reconnectDelay);

    // Exponential backoff with max 30s
    this.reconnectDelay = Math.min(this.reconnectDelay * 2, 30000);
  }

  private clearReconnectTimer(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }

  private startHeartbeat(): void {
    this.stopHeartbeat();
    // Send ping every 30 seconds to keep connection alive
    this.heartbeatInterval = setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify({ type: 'ping' }));
      }
    }, 30000);
  }

  private stopHeartbeat(): void {
    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval);
      this.heartbeatInterval = null;
    }
  }

  private flushMessageQueue(): void {
    if (this.messageQueue.length === 0) {
      return;
    }
    
    console.log(`[WebSocket] Flushing ${this.messageQueue.length} queued messages`);
    
    while (this.messageQueue.length > 0) {
      const message = this.messageQueue.shift();
      if (message && this.ws?.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify(message));
      }
    }
  }
}

// Export singleton instance
export const wsService = new WebSocketService();

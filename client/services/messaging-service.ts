// Abstract Messaging Service - Switches between WebSocket and XMPP
import { wsService, WS_EVENTS } from './websocket';
import xmppClient from './xmpp-client';

export type ProtocolType = 'websocket' | 'xmpp';

interface XMPPConfig {
  username: string;
  password: string;
  server: string;
}

class MessagingService {
  private currentProtocol: ProtocolType = 'websocket';
  private xmppConfig: XMPPConfig | null = null;
  private listeners: Map<string, ((data: any) => void)[]> = new Map();

  async connect(protocol: ProtocolType, config: XMPPConfig): Promise<void> {
    this.currentProtocol = protocol;
    this.xmppConfig = config;

    if (protocol === 'websocket') {
      // Disconnect XMPP if connected
      xmppClient.disconnect();
      
      // Connect via WebSocket
      await wsService.connect();
      this.emit('connected', { protocol: 'websocket' });
    } else if (protocol === 'xmpp') {
      // Disconnect WebSocket
      wsService.disconnect();
      
      // Connect via XMPP
      await xmppClient.connect(
        config.username,
        config.password,
        config.server
      );
      this.emit('connected', { protocol: 'xmpp' });
    }
  }

  disconnect(): void {
    if (this.currentProtocol === 'websocket') {
      wsService.disconnect();
    } else if (this.currentProtocol === 'xmpp') {
      xmppClient.disconnect();
    }
    this.emit('disconnected', null);
  }

  sendMessage(chatId: string, content: string, type: string = 'text'): void {
    if (this.currentProtocol === 'websocket') {
      wsService.send('message', {
        chat_id: chatId,
        content,
        message_type: type,
      });
    } else if (this.currentProtocol === 'xmpp') {
      // For XMPP, we send to the room JID
      if (this.xmppConfig) {
        const roomJID = `${chatId}@conference.${this.xmppConfig.server}`;
        xmppClient.sendMessage(roomJID, content);
      }
    }
  }

  joinChat(chatId: string): void {
    if (this.currentProtocol === 'websocket') {
      wsService.send('join_chat', { chat_id: chatId });
    } else if (this.currentProtocol === 'xmpp') {
      if (this.xmppConfig) {
        const roomJID = `${chatId}@conference.${this.xmppConfig.server}`;
        xmppClient.joinRoom(roomJID);
      }
    }
  }

  leaveChat(chatId: string): void {
    if (this.currentProtocol === 'websocket') {
      wsService.send('leave_chat', { chat_id: chatId });
    } else if (this.currentProtocol === 'xmpp') {
      if (this.xmppConfig) {
        const roomJID = `${chatId}@conference.${this.xmppConfig.server}`;
        xmppClient.leaveRoom(roomJID);
      }
    }
  }

  sendTyping(chatId: string, isTyping: boolean): void {
    if (this.currentProtocol === 'websocket') {
      wsService.send('typing', {
        chat_id: chatId,
        is_typing: isTyping,
      });
    }
    // XMPP typing would need to be implemented
  }

  on(event: string, callback: (data: any) => void): void {
    // Subscribe to both WebSocket and XMPP events
    if (!this.listeners.has(event)) {
      this.listeners.set(event, []);
    }
    this.listeners.get(event)!.push(callback);

    // Subscribe to WebSocket events
    if (event === WS_EVENTS.NEW_MESSAGE) {
      wsService.onMessage(callback);
    } else if (event === 'connected') {
      wsService.onConnect(() => callback({ protocol: this.currentProtocol }));
    } else if (event === 'disconnected') {
      wsService.onDisconnect((reason) => callback({ reason }));
    }

    // Subscribe to XMPP events (map XMPP events to our event names)
    if (event === WS_EVENTS.NEW_MESSAGE) {
      xmppClient.on('messageReceived', callback);
    } else if (event === 'connected') {
      xmppClient.on('connected', callback);
    } else if (event === 'disconnected') {
      xmppClient.on('disconnected', callback);
    }
  }

  off(event: string, callback: (data: any) => void): void {
    const callbacks = this.listeners.get(event);
    if (callbacks) {
      const index = callbacks.indexOf(callback);
      if (index > -1) {
        callbacks.splice(index, 1);
      }
    }

    // Unsubscribe from XMPP
    xmppClient.off(event, callback);
  }

  getCurrentProtocol(): ProtocolType {
    return this.currentProtocol;
  }

  private emit(event: string, data: any): void {
    const callbacks = this.listeners.get(event);
    if (callbacks) {
      callbacks.forEach(callback => callback(data));
    }
  }
}

// Singleton instance
const messagingService = new MessagingService();

export default messagingService;

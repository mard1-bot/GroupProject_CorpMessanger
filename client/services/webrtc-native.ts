import { wsService } from './websocket';
import type { Chat, User } from '@/types/chat';

export type CallType = 'audio' | 'video';

export interface CallState {
  callId: string;
  chatId: string;
  callerId: string;
  calleeId: string | null;
  type: CallType;
  isCaller: boolean;
  isConnected: boolean;
  isRinging: boolean;
  isEnded: boolean;
  error?: string;
}

class NativeCallService {
  private currentCall: CallState | null = null;
  private onStateChangeCallbacks: ((state: CallState | null) => void)[] = [];
  private onLocalStreamCallbacks: ((stream: any) => void)[] = [];
  private onRemoteStreamCallbacks: ((stream: any) => void)[] = [];

  constructor() {
    this.setupWebSocketListeners();
  }

  private setupWebSocketListeners() {
    wsService.onMessage((message) => {
      switch (message.type) {
        case 'call_offer':
        case 'call_join':
        case 'call_answer':
        case 'call_end':
        case 'call_reject':
        case 'call_ringing':
        case 'call_busy':
          // Handle cleanup if we receive end signals
          if (message.type === 'call_end' || message.type === 'call_reject' || message.type === 'call_busy') {
            this.handleCallEnd();
          }
          break;
      }
    });
  }

  private notifyStateChange() {
    const state = this.currentCall ? { ...this.currentCall } : null;
    this.onStateChangeCallbacks.forEach(cb => cb(state));
  }

  onStateChange(callback: (state: CallState | null) => void) {
    this.onStateChangeCallbacks.push(callback);
    return () => {
      this.onStateChangeCallbacks = this.onStateChangeCallbacks.filter(cb => cb !== callback);
    };
  }

  onLocalStream(callback: (stream: any) => void) {
    this.onLocalStreamCallbacks.push(callback);
    return () => {
      this.onLocalStreamCallbacks = this.onLocalStreamCallbacks.filter(cb => cb !== callback);
    };
  }

  onRemoteStream(callback: (stream: any) => void) {
    this.onRemoteStreamCallbacks.push(callback);
    return () => {
      this.onRemoteStreamCallbacks = this.onRemoteStreamCallbacks.filter(cb => cb !== callback);
    };
  }

  async startCall(chatId: string, calleeId: string | null, type: CallType): Promise<void> {
    alert('Звонки в мобильном приложении временно недоступны (требуется установка @livekit/react-native). Пожалуйста, используйте веб-версию.');
    throw new Error('LiveKit RN SDK required');
  }

  async joinCall(callId: string, chatId: string): Promise<void> {
    alert('Звонки в мобильном приложении временно недоступны (требуется установка @livekit/react-native). Пожалуйста, используйте веб-версию.');
    throw new Error('LiveKit RN SDK required');
  }

  async acceptCall(callId: string, chatId: string): Promise<void> {
    alert('Звонки в мобильном приложении временно недоступны (требуется установка @livekit/react-native). Пожалуйста, используйте веб-версию.');
    throw new Error('LiveKit RN SDK required');
  }

  rejectCall(callId: string) {
    wsService.send('call_reject', { call_id: callId });
    this.endCall();
  }

  endCall() {
    if (this.currentCall) {
      wsService.send('call_end', { call_id: this.currentCall.callId });
    }
    this.cleanup();
  }

  private cleanup() {
    this.currentCall = null;
    this.notifyStateChange();
  }

  private handleCallEnd() {
    if (this.currentCall) {
      this.currentCall.isEnded = true;
      this.notifyStateChange();
    }
    this.cleanup();
  }

  getCurrentCall(): CallState | null {
    return this.currentCall;
  }

  toggleMute(): boolean {
    return false;
  }

  toggleVideo(): boolean {
    return false;
  }
}

export const nativeCallService = new NativeCallService();

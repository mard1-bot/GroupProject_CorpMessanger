import {
  RTCPeerConnection,
  RTCIceCandidate,
  RTCSessionDescription,
  MediaStream,
  mediaDevices,
} from 'react-native-webrtc';
import { wsService } from './websocket';
import { api } from './api';
import type { Chat, User } from '@/types/chat';

export type CallType = 'audio' | 'video';

export interface CallState {
  callId: string;
  chatId: string;
  callerId: string;
  calleeId: string;
  type: CallType;
  isCaller: boolean;
  isConnected: boolean;
  isRinging: boolean;
  isEnded: boolean;
  error?: string;
}

class NativeCallService {
  private pc: RTCPeerConnection | null = null;
  private localStream: MediaStream | null = null;
  private remoteStream: MediaStream | null = null;
  private currentCall: CallState | null = null;
  private onStateChangeCallbacks: ((state: CallState | null) => void)[] = [];
  private onLocalStreamCallbacks: ((stream: MediaStream) => void)[] = [];
  private onRemoteStreamCallbacks: ((stream: MediaStream) => void)[] = [];

  private configuration = {
    iceServers: [
      { urls: 'stun:stun.l.google.com:19302' },
      { urls: 'stun:stun1.l.google.com:19302' },
    ],
  };

  constructor() {
    this.setupWebSocketListeners();
  }

  private setupWebSocketListeners() {
    wsService.onMessage((message) => {
      switch (message.type) {
        case 'call_offer':
          if (this.currentCall?.isCaller && !this.currentCall?.callId && message.payload?.call_id) {
            this.currentCall.callId = message.payload.call_id;
            this.currentCall.callerId = message.payload.caller_id;
            this.notifyStateChange();
          } else {
            this.handleIncomingCall(message.payload);
          }
          break;
        case 'call_answer':
          this.handleAnswer(message.payload);
          break;
        case 'call_ice':
          this.handleIceCandidate(message.payload);
          break;
        case 'call_end':
          this.handleCallEnd();
          break;
        case 'call_reject':
          this.handleCallRejected();
          break;
        case 'call_ringing':
          this.handleRinging();
          break;
        case 'call_busy':
          this.handleCallBusy();
          break;
      }
    });
  }

  private notifyStateChange() {
    this.onStateChangeCallbacks.forEach(cb => cb(this.currentCall));
  }

  onStateChange(callback: (state: CallState | null) => void) {
    this.onStateChangeCallbacks.push(callback);
    return () => {
      this.onStateChangeCallbacks = this.onStateChangeCallbacks.filter(cb => cb !== callback);
    };
  }

  onLocalStream(callback: (stream: MediaStream) => void) {
    this.onLocalStreamCallbacks.push(callback);
    if (this.localStream) {
      callback(this.localStream);
    }
    return () => {
      this.onLocalStreamCallbacks = this.onLocalStreamCallbacks.filter(cb => cb !== callback);
    };
  }

  onRemoteStream(callback: (stream: MediaStream) => void) {
    this.onRemoteStreamCallbacks.push(callback);
    if (this.remoteStream) {
      callback(this.remoteStream);
    }
    return () => {
      this.onRemoteStreamCallbacks = this.onRemoteStreamCallbacks.filter(cb => cb !== callback);
    };
  }

  private createPeerConnection(): RTCPeerConnection {
    const pc = new RTCPeerConnection(this.configuration);

    (pc as any).addEventListener('icecandidate', (event: any) => {
      if (event.candidate && this.currentCall) {
        wsService.send('call_ice', {
          call_id: this.currentCall.callId,
          candidate: event.candidate.candidate,
          sdp_mline_index: event.candidate.sdpMLineIndex,
          sdp_mid: event.candidate.sdpMid,
        });
      }
    });

    (pc as any).addEventListener('addstream', (event: any) => {
      this.remoteStream = event.stream;
      this.onRemoteStreamCallbacks.forEach(cb => cb(event.stream));
    });

    (pc as any).addEventListener('connectionstatechange', () => {
      if (this.currentCall) {
        this.currentCall.isConnected = pc.connectionState === 'connected';
        this.notifyStateChange();
      }
    });

    return pc;
  }

  async startCall(chatId: string, calleeId: string, type: CallType): Promise<void> {
    try {
      const isVideoCall = type === 'video';
      
      try {
        this.localStream = await mediaDevices.getUserMedia({
          audio: true,
          video: isVideoCall,
        });
      } catch (mediaError: any) {
        if (isVideoCall) {
          console.warn('Video access failed, falling back to audio-only:', mediaError.message);
          try {
            this.localStream = await mediaDevices.getUserMedia({
              audio: true,
              video: false,
            });
            type = 'audio';
          } catch (audioError: any) {
            console.error('Audio access also failed:', audioError.message);
            throw new Error('Не удалось получить доступ к микрофону и камере. Проверьте разрешения.');
          }
        } else {
          throw new Error('Не удалось получить доступ к микрофону. Проверьте разрешения.');
        }
      }
      
      this.onLocalStreamCallbacks.forEach(cb => cb(this.localStream!));

      this.pc = this.createPeerConnection();

      this.localStream.getTracks().forEach((track) => {
        this.pc!.addTrack(track, this.localStream!);
      });

      const offer = await this.pc.createOffer();
      await this.pc.setLocalDescription(offer);

      wsService.send('call_offer', {
        chat_id: chatId,
        callee_id: calleeId,
        type: type,
        sdp: offer.sdp,
      });

      this.currentCall = {
        callId: '',
        chatId,
        callerId: '',
        calleeId,
        type,
        isCaller: true,
        isConnected: false,
        isRinging: false,
        isEnded: false,
      };
      this.notifyStateChange();
    } catch (error) {
      console.error('Failed to start call:', error);
      throw error;
    }
  }

  async acceptCall(callId: string, chatId: string): Promise<void> {
    try {
      if (!this.pc) {
        this.pc = this.createPeerConnection();
      }

      if (!this.localStream) {
        const isVideoCall = this.currentCall?.type === 'video';
        
        try {
          this.localStream = await mediaDevices.getUserMedia({
            audio: true,
            video: isVideoCall,
          });
        } catch (mediaError: any) {
          if (isVideoCall) {
            console.warn('Video access failed, falling back to audio-only:', mediaError.message);
            try {
              this.localStream = await mediaDevices.getUserMedia({
                audio: true,
                video: false,
              });
              if (this.currentCall) {
                this.currentCall.type = 'audio';
              }
            } catch (audioError: any) {
              console.error('Audio access also failed:', audioError.message);
              throw new Error('Не удалось получить доступ к микрофону и камере. Проверьте разрешения.');
            }
          } else {
            throw new Error('Не удалось получить доступ к микрофону. Проверьте разрешения.');
          }
        }
        
        this.onLocalStreamCallbacks.forEach(cb => cb(this.localStream!));

        this.localStream.getTracks().forEach((track) => {
          this.pc!.addTrack(track, this.localStream!);
        });
      }

      wsService.send('call_accept', { call_id: callId });

      if (this.currentCall) {
        this.currentCall.isRinging = true;
        this.notifyStateChange();
      }

      const answer = await this.pc.createAnswer();
      await this.pc.setLocalDescription(answer);

      wsService.send('call_answer', {
        call_id: callId,
        sdp: answer.sdp,
      });

      if (this.currentCall) {
        this.currentCall.isRinging = false;
        this.currentCall.isConnected = true;
        this.notifyStateChange();
      }
    } catch (error) {
      console.error('Failed to accept call:', error);
      throw error;
    }
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
    this.localStream?.getTracks().forEach(track => track.stop());
    this.remoteStream?.getTracks().forEach(track => track.stop());
    this.pc?.close();

    this.localStream = null;
    this.remoteStream = null;
    this.pc = null;
    this.currentCall = null;

    this.notifyStateChange();
  }

  private handleIncomingCall(payload: any) {
    this.currentCall = {
      callId: payload.call_id,
      chatId: payload.chat_id,
      callerId: payload.caller_id,
      calleeId: '',
      type: payload.type || 'audio',
      isCaller: false,
      isConnected: false,
      isRinging: false,
      isEnded: false,
    };

    this.pc = this.createPeerConnection();

    this.pc.setRemoteDescription(new RTCSessionDescription({
      type: 'offer',
      sdp: payload.sdp,
    })).catch(err => console.error('Failed to set remote description:', err));

    this.notifyStateChange();
  }

  private async handleAnswer(payload: any) {
    if (!this.pc) return;

    try {
      await this.pc.setRemoteDescription(new RTCSessionDescription({
        type: 'answer',
        sdp: payload.sdp,
      }));

      if (this.currentCall) {
        this.currentCall.isConnected = true;
        this.notifyStateChange();
      }
    } catch (error) {
      console.error('Failed to handle answer:', error);
    }
  }

  private async handleIceCandidate(payload: any) {
    if (!this.pc) return;

    try {
      await this.pc.addIceCandidate(new RTCIceCandidate({
        candidate: payload.candidate,
        sdpMLineIndex: payload.sdp_mline_index,
        sdpMid: payload.sdp_mid,
      }));
    } catch (error) {
      console.error('Failed to add ICE candidate:', error);
    }
  }

  private handleCallEnd() {
    if (this.currentCall) {
      this.currentCall.isEnded = true;
      this.notifyStateChange();
    }
    this.cleanup();
  }

  private handleCallRejected() {
    if (this.currentCall) {
      this.currentCall.isEnded = true;
      this.currentCall.error = 'Звонок отклонен';
      this.notifyStateChange();
    }
    this.cleanup();
  }

  private handleRinging() {
    if (this.currentCall) {
      this.currentCall.isRinging = true;
      this.notifyStateChange();
    }
  }

  private handleCallBusy() {
    if (this.currentCall) {
      this.currentCall.isEnded = true;
      this.currentCall.error = 'Абонент занят';
      this.notifyStateChange();
    }
    this.cleanup();
  }

  getCurrentCall(): CallState | null {
    return this.currentCall;
  }

  toggleMute(): boolean {
    if (!this.localStream) return false;
    
    const audioTrack = this.localStream.getAudioTracks()[0];
    if (audioTrack) {
      audioTrack.enabled = !audioTrack.enabled;
      return audioTrack.enabled;
    }
    return false;
  }

  toggleVideo(): boolean {
    if (!this.localStream) return false;
    
    const videoTrack = this.localStream.getVideoTracks()[0];
    if (videoTrack) {
      videoTrack.enabled = !videoTrack.enabled;
      return videoTrack.enabled;
    }
    return false;
  }
}

export const nativeCallService = new NativeCallService();

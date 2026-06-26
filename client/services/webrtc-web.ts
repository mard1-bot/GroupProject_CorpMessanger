import { wsService } from './websocket';
import { api } from './api';
import type { Chat, User } from '@/types/chat';

// Import types only (removed at compile time)
import type { Room as RoomType, RoomEvent as RoomEventType, Track as TrackType, RemoteTrackPublication as RemoteTrackPublicationType, RemoteParticipant as RemoteParticipantType } from 'livekit-client';

// LiveKit runtime values - loaded dynamically on web only
let Room: typeof RoomType | undefined;
let RoomEvent: typeof RoomEventType | undefined;
let Track: typeof TrackType | undefined;
let RemoteTrackPublication: typeof RemoteTrackPublicationType | undefined;
let RemoteParticipant: typeof RemoteParticipantType | undefined;

// Load LiveKit only on web platform
if (typeof window !== 'undefined' && typeof document !== 'undefined') {
  try {
    const livekit = require('livekit-client');
    Room = livekit.Room;
    RoomEvent = livekit.RoomEvent;
    Track = livekit.Track;
    RemoteTrackPublication = livekit.RemoteTrackPublication;
    RemoteParticipant = livekit.RemoteParticipant;
  } catch (e) {
    console.warn('[WebRTC] LiveKit not available');
  }
}

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
  liveKitRoom?: string;
  liveKitURL?: string;
  liveKitToken?: string;
  offerSdp?: string;
  participants?: any[];
  isGroup?: boolean;
}

class WebCallService {
  private localStream: MediaStream | null = null;
  private currentCall: CallState | null = null;
  private onStateChangeCallbacks: ((state: CallState | null) => void)[] = [];
  private onLocalStreamCallbacks: ((stream: MediaStream) => void)[] = [];
  private onRemoteStreamCallbacks: ((stream: MediaStream) => void)[] = [];
  private onParticipantJoinedCallbacks: ((participant: any) => void)[] = [];
  private onParticipantLeftCallbacks: ((participant: any) => void)[] = [];
  private onParticipantStreamCallbacks: ((identity: string, stream: MediaStream) => void)[] = [];
  private liveKitRoom: any = null;
  private remoteStreams: Map<string, MediaStream> = new Map();

  constructor() {
    this.setupWebSocketListeners();
  }

  private setupWebSocketListeners() {
    wsService.onMessage((message) => {
      const payload = message.payload;
      switch (message.type) {
        case 'call_error':
          console.error('[WebRTC] Call error received:', message.payload);
          alert('Ошибка звонка: ' + (message.payload.message || message.payload.error));
          this.handleCallEnd();
          break;
        case 'call_offer':
          if (this.currentCall?.isCaller) {
            if (!this.currentCall?.callId && payload?.call_id) {
              // This is our own offer being confirmed by the server
              this.currentCall.callId = payload.call_id;
              this.currentCall.callerId = payload.caller_id;
              
              if (payload.livekit) {
                this.currentCall.liveKitRoom = payload.livekit.room_name;
                this.currentCall.liveKitURL = payload.livekit.url;
                this.currentCall.liveKitToken = payload.livekit.token;
                
                this.connectToLiveKitRoom(
                  payload.livekit.room_name,
                  payload.livekit.url,
                  payload.livekit.token
                ).catch(e => console.error('Failed to connect to LiveKit:', e));
              }

              this.notifyStateChange();
            }
            // Ignore any reflected or duplicate call_offers if we are already the caller
          } else {
            // Incoming call
            this.handleIncomingCall(payload);
          }
          break;
        case 'call_join':
          this.handleJoinResponse(message.payload);
          break;
        case 'call_answer':
          this.handleAnswer(message.payload);
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
    return () => {
      this.onRemoteStreamCallbacks = this.onRemoteStreamCallbacks.filter(cb => cb !== callback);
    };
  }

  onParticipantJoined(callback: (participant: RemoteParticipant) => void) {
    this.onParticipantJoinedCallbacks.push(callback);
    return () => {
      this.onParticipantJoinedCallbacks = this.onParticipantJoinedCallbacks.filter(cb => cb !== callback);
    };
  }

  onParticipantLeft(callback: (participant: RemoteParticipant) => void) {
    this.onParticipantLeftCallbacks.push(callback);
    return () => {
      this.onParticipantLeftCallbacks = this.onParticipantLeftCallbacks.filter(cb => cb !== callback);
    };
  }

  onParticipantStream(callback: (identity: string, stream: MediaStream) => void) {
    this.onParticipantStreamCallbacks.push(callback);
    // Notify about existing streams
    this.remoteStreams.forEach((stream, identity) => {
      callback(identity, stream);
    });
    return () => {
      this.onParticipantStreamCallbacks = this.onParticipantStreamCallbacks.filter(cb => cb !== callback);
    };
  }



  async startCall(chatId: string, calleeId: string | null, type: CallType): Promise<void> {
    try {
      const isVideoCall = type === 'video';
      console.log('[WebRTC] Starting call:', { chatId, calleeId, type, isVideoCall });
      
      try {
        this.localStream = await navigator.mediaDevices.getUserMedia({
          audio: true,
          video: isVideoCall,
        });
        console.log('[WebRTC] Local stream obtained:', {
          hasAudio: this.localStream.getAudioTracks().length > 0,
          hasVideo: this.localStream.getVideoTracks().length > 0,
          videoTracks: this.localStream.getVideoTracks().map(t => ({ id: t.id, enabled: t.enabled, label: t.label }))
        });
        this.onLocalStreamCallbacks.forEach(cb => cb(this.localStream!));
      } catch (mediaError: any) {
        console.error('[WebRTC] Media access failed:', mediaError);
        if (isVideoCall) {
          console.warn('Video access failed, falling back to audio-only:', mediaError.message);
          try {
            this.localStream = await navigator.mediaDevices.getUserMedia({
              audio: true,
              video: false,
            });
            type = 'audio';
            console.log('[WebRTC] Fallback to audio-only successful');
            if (this.localStream) {
              this.onLocalStreamCallbacks.forEach(cb => cb(this.localStream!));
            }
          } catch (audioError: any) {
            console.error('Audio access also failed:', audioError.message);
            throw new Error('Не удалось получить доступ к микрофону и камере. Проверьте разрешения браузера.');
          }
        } else {
          throw new Error('Не удалось получить доступ к микрофону. Проверьте разрешения браузера.');
        }
      }
      
      if (this.localStream) {
        this.onLocalStreamCallbacks.forEach(cb => cb(this.localStream!));
      }

      wsService.send('call_offer', {
        chat_id: chatId,
        callee_id: calleeId,
        type: type,
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

  private async connectToLiveKitRoom(roomName: string, url: string, token: string) {
    console.log('[LiveKit] Connecting to room:', roomName);

    if (!Room || !RoomEvent) {
      console.error('[LiveKit] LiveKit not available');
      return;
    }

    const room = new Room();
    
    room.on(RoomEvent.TrackSubscribed, (track: any, publication: any, participant: any) => {
      console.log('[LiveKit] Track subscribed:', track.kind, 'from', participant.identity);
      if (track.kind === 'video' || track.kind === 'audio') {
        let stream = this.remoteStreams.get(participant.identity);
        if (!stream) {
          stream = new MediaStream();
          this.remoteStreams.set(participant.identity, stream);
        }
        stream.addTrack(track.mediaStreamTrack!);
        
        // Use a slight timeout to batch multiple rapid track events
        setTimeout(() => {
          this.onRemoteStreamCallbacks.forEach(cb => cb(stream!));
          this.onParticipantStreamCallbacks.forEach(cb => cb(participant.identity, stream!));
        }, 50);
      }
    });

    room.on(RoomEvent.TrackUnsubscribed, (track: any, publication: any, participant: any) => {
      console.log('[LiveKit] Track unsubscribed:', track.kind, 'from', participant.identity);
      this.remoteStreams.delete(participant.identity);
    });

    room.on(RoomEvent.ParticipantConnected, (participant: any) => {
      console.log('[LiveKit] Participant joined:', participant.identity);
      this.onParticipantJoinedCallbacks.forEach(cb => cb(participant));
      if (this.currentCall) {
        this.currentCall.participants = Array.from(room.remoteParticipants.values());
        this.notifyStateChange();
      }
    });

    room.on(RoomEvent.ParticipantDisconnected, (participant: any) => {
      console.log('[LiveKit] Participant left:', participant.identity);
      this.onParticipantLeftCallbacks.forEach(cb => cb(participant));
      this.remoteStreams.delete(participant.identity);
      if (this.currentCall) {
        this.currentCall.participants = Array.from(room.remoteParticipants.values());
        this.notifyStateChange();
      }
    });

    room.on(RoomEvent.Connected, () => {
      console.log('[LiveKit] Connected to room');
      if (this.currentCall) {
        this.currentCall.isConnected = true;
        this.notifyStateChange();
      }
    });

    room.on(RoomEvent.Disconnected, () => {
      console.log('[LiveKit] Disconnected from room');
      this.cleanup();
    });

    await room.connect(url, token);
    
    if (this.localStream) {
      for (const track of this.localStream.getTracks()) {
        await room.localParticipant.publishTrack(track);
      }
    }

    this.liveKitRoom = room;
    
    if (this.currentCall) {
      this.currentCall.participants = Array.from(room.remoteParticipants.values());
      this.notifyStateChange();
    }

    console.log('[LiveKit] Room connection successful');
  }

  
  async joinCall(callId: string, chatId: string): Promise<void> {
    console.log('[WebRTC] Joining active call:', callId);
    try {
      if (this.currentCall) {
        this.currentCall.isRinging = false;
      } else {
        this.currentCall = {
          callId,
          chatId,
          callerId: '',
          calleeId: null,
          type: 'audio', // Will be updated from response
          isCaller: false,
          isConnected: false,
          isRinging: false,
          isEnded: false,
        };
      }
      this.notifyStateChange();
      
      wsService.send('call_join', {
        call_id: callId
      });
    } catch (error) {
      console.error('Failed to send join request:', error);
      throw error;
    }
  }

  private async handleJoinResponse(payload: any) {
    console.log('[WebRTC] Received join response:', payload);
    if (!this.currentCall) return;
    
    // Update callId if we are the caller and don't have it yet
    if (this.currentCall.isCaller && !this.currentCall.callId && payload.call_id) {
      this.currentCall.callId = payload.call_id;
    } else if (payload.call_id !== this.currentCall.callId) {
      // Ignore join response for different calls
      return;
    }

    if (payload.type) {
      this.currentCall.type = payload.type;
    }
    this.notifyStateChange();
    
    if (payload.livekit) {
      this.currentCall.liveKitRoom = payload.livekit.room_name;
      this.currentCall.liveKitURL = payload.livekit.url;
      this.currentCall.liveKitToken = payload.livekit.token;
      
      const isVideoCall = this.currentCall.type === 'video';
      try {
        this.localStream = await navigator.mediaDevices.getUserMedia({
          audio: true,
          video: isVideoCall,
        });
        this.onLocalStreamCallbacks.forEach(cb => cb(this.localStream!));
      } catch (e) {
        console.error('Media error on join:', e);
      }
      
      this.connectToLiveKitRoom(
        payload.livekit.room_name,
        payload.livekit.url,
        payload.livekit.token
      ).then(() => {
        if (this.currentCall) {
          this.currentCall.isConnected = true;
          this.notifyStateChange();
        }
      }).catch(e => console.error('Failed to connect to LiveKit:', e));
    } else {
      console.error('[WebRTC] LiveKit is required for group calls, fallback not supported for join yet');
    }
  }

  async acceptCall(callId: string, chatId: string): Promise<void> {
    console.log('[WebRTC] acceptCall called with callId:', callId, 'chatId:', chatId);
    console.log('[WebRTC] acceptCall currentCall state:', {
      hasCurrentCall: !!this.currentCall,
      isGroup: this.currentCall?.isGroup,
      hasLiveKitRoom: !!this.currentCall?.liveKitRoom,
      hasLiveKitURL: !!this.currentCall?.liveKitURL,
      hasLiveKitToken: !!this.currentCall?.liveKitToken,
      liveKitRoom: this.currentCall?.liveKitRoom,
    });
    try {
      if (!this.localStream) {
        const isVideoCall = this.currentCall?.type === 'video';
        console.log('[WebRTC] acceptCall: requesting media, isVideoCall:', isVideoCall);
        try {
          this.localStream = await navigator.mediaDevices.getUserMedia({
            audio: true,
            video: isVideoCall,
          });
          console.log('[WebRTC] acceptCall: media obtained successfully');
          this.onLocalStreamCallbacks.forEach(cb => cb(this.localStream!));
        } catch (mediaError: any) {
          console.error('[WebRTC] acceptCall: media access error:', mediaError);
          if (isVideoCall) {
            try {
              console.log('[WebRTC] acceptCall: falling back to audio-only');
              this.localStream = await navigator.mediaDevices.getUserMedia({
                audio: true,
                video: false,
              });
              if (this.currentCall) {
                this.currentCall.type = 'audio';
              }
              console.log('[WebRTC] acceptCall: audio-only fallback success');
              this.onLocalStreamCallbacks.forEach(cb => cb(this.localStream!));
            } catch (audioError: any) {
              throw new Error('Не удалось получить доступ к микрофону и камере. Проверьте разрешения браузера.');
            }
          } else {
            throw new Error('Не удалось получить доступ к микрофону. Проверьте разрешения браузера.');
          }
        }
      } else {
        console.log('[WebRTC] acceptCall: localStream already exists');
      }

      // Use LiveKit for all calls
      if (this.currentCall?.isGroup) {
        console.log('[WebRTC] Group call, sending call_join to get LiveKit credentials');
        wsService.send('call_join', { call_id: callId, chat_id: this.currentCall.chatId });
        
        if (this.currentCall) {
          this.currentCall.isConnected = true;
          this.notifyStateChange();
        }
        return;
      } else if (this.currentCall?.liveKitRoom && this.currentCall?.liveKitURL && this.currentCall?.liveKitToken) {
        console.log('[WebRTC] acceptCall: connecting to LiveKit room:', this.currentCall.liveKitRoom);
        await this.connectToLiveKitRoom(
          this.currentCall.liveKitRoom,
          this.currentCall.liveKitURL,
          this.currentCall.liveKitToken
        );
        console.log('[WebRTC] acceptCall: LiveKit connected, sending call_accept');

        wsService.send('call_accept', { call_id: callId });

        if (this.currentCall) {
          this.currentCall.isRinging = false;
          this.currentCall.isConnected = true;
          this.notifyStateChange();
        }
        console.log('[WebRTC] acceptCall: call accepted and connected!');
        return;
      } else {
        console.error('[WebRTC] acceptCall: No LiveKit credentials available!', {
          liveKitRoom: this.currentCall?.liveKitRoom,
          liveKitURL: this.currentCall?.liveKitURL,
          liveKitToken: this.currentCall?.liveKitToken ? 'present' : 'missing',
        });
        throw new Error('Server did not provide LiveKit credentials.');
      }
    } catch (error) {
      console.error('[WebRTC] Failed to accept call:', error);
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
    this.remoteStreams.forEach(stream => stream.getTracks().forEach(track => track.stop()));
    this.remoteStreams.clear();

    try {
      if (this.liveKitRoom) {
        this.liveKitRoom.disconnect();
        this.liveKitRoom = null;
      }
    } catch (e) {
      console.error('[WebRTC] Error during cleanup:', e);
    }

    this.localStream = null;
    this.currentCall = null;

    this.notifyStateChange();
  }

  private handleIncomingCall(payload: any) {
    console.log('[WebRTC] handleIncomingCall payload:', {
      callId: payload.call_id,
      chatId: payload.chat_id,
      type: payload.type,
      hasSdp: !!payload.sdp,
      sdpLength: payload.sdp?.length,
      hasLivekit: !!payload.livekit,
      livekit: payload.livekit
    });

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
      isGroup: payload.is_group === true,
      liveKitRoom: payload.livekit?.room_name,
      liveKitURL: payload.livekit?.url,
      liveKitToken: payload.livekit?.token,
      offerSdp: payload.sdp,
    };

    // Wait for the user to answer via UI
    this.notifyStateChange();
  }

  private async handleAnswer(payload: any) {
    if (this.currentCall) {
      this.currentCall.isConnected = true;
      this.currentCall.isRinging = false;
      this.notifyStateChange();
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

export const webCallService = new WebCallService();

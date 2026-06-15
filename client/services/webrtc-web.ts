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
}

class WebCallService {
  private pc: RTCPeerConnection | null = null;
  private localStream: MediaStream | null = null;
  private remoteStream: MediaStream | null = null;
  private currentCall: CallState | null = null;
  private onStateChangeCallbacks: ((state: CallState | null) => void)[] = [];
  private onLocalStreamCallbacks: ((stream: MediaStream) => void)[] = [];
  private onRemoteStreamCallbacks: ((stream: MediaStream) => void)[] = [];
  private onParticipantJoinedCallbacks: ((participant: any) => void)[] = [];
  private onParticipantLeftCallbacks: ((participant: any) => void)[] = [];
  private onParticipantStreamCallbacks: ((identity: string, stream: MediaStream) => void)[] = [];
  private liveKitRoom: any = null;
  private remoteStreams: Map<string, MediaStream> = new Map();
  private pendingIceCandidates: { candidate: string; sdp_mline_index: number | null; sdp_mid: string | null }[] = [];

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
            // Flush buffered ICE candidates now that we have the callId
            this.flushPendingIceCandidates();
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

  private flushPendingIceCandidates() {
    if (!this.currentCall?.callId) return;
    const callId = this.currentCall.callId;
    for (const ice of this.pendingIceCandidates) {
      wsService.send('call_ice', {
        call_id: callId,
        candidate: ice.candidate,
        sdp_mline_index: ice.sdp_mline_index,
        sdp_mid: ice.sdp_mid,
      });
    }
    this.pendingIceCandidates = [];
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

  private createPeerConnection() {
    // Configure ICE servers with STUN and TURN
    const iceServers: RTCIceServer[] = [
      // Google STUN servers
      { urls: 'stun:stun.l.google.com:19302' },
      { urls: 'stun:stun1.l.google.com:19302' },
      { urls: 'stun:stun2.l.google.com:19302' },
      { urls: 'stun:stun3.l.google.com:19302' },
      { urls: 'stun:stun4.l.google.com:19302' },
      // Mozilla STUN servers
      { urls: 'stun:stun.services.mozilla.com' },
      // Twilio STUN servers (free)
      { urls: 'stun:global.stun.twilio.com:3478' },
    ];

    // Add TURN server if configured
    const turnServerUri = process.env.EXPO_PUBLIC_TURN_SERVER_URI;
    const turnUsername = process.env.EXPO_PUBLIC_TURN_USERNAME;
    const turnPassword = process.env.EXPO_PUBLIC_TURN_PASSWORD;

    if (turnServerUri && turnUsername && turnPassword) {
      iceServers.push({
        urls: turnServerUri,
        username: turnUsername,
        credential: turnPassword,
      });
      console.log('[WebRTC] Using TURN server:', turnServerUri);
    } else {
      console.warn('[WebRTC] TURN server not configured - calls may fail behind NAT. For production, configure a TURN server.');
    }

    const pc = new RTCPeerConnection({ iceServers });

    pc.onicecandidate = (event) => {
      if (event.candidate && this.currentCall) {
        if (this.currentCall.callId) {
          wsService.send('call_ice', {
            call_id: this.currentCall.callId,
            candidate: event.candidate.candidate,
            sdp_mline_index: event.candidate.sdpMLineIndex,
            sdp_mid: event.candidate.sdpMid,
          });
        } else {
          // Buffer ICE candidates until callId is assigned
          this.pendingIceCandidates.push({
            candidate: event.candidate.candidate,
            sdp_mline_index: event.candidate.sdpMLineIndex,
            sdp_mid: event.candidate.sdpMid,
          });
        }
      }
    };

    pc.ontrack = (event) => {
      this.remoteStream = event.streams[0];
      this.onRemoteStreamCallbacks.forEach(cb => cb(event.streams[0]));
    };

    pc.onconnectionstatechange = () => {
      if (this.currentCall) {
        this.currentCall.isConnected = pc.connectionState === 'connected';
        this.notifyStateChange();
      }
    };

    return pc;
  }

  async startCall(chatId: string, calleeId: string, type: CallType): Promise<void> {
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

      this.pc = this.createPeerConnection();

      this.localStream.getTracks().forEach(track => {
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
        const stream = new MediaStream();
        stream.addTrack(track.mediaStreamTrack!);
        this.remoteStreams.set(participant.identity, stream);
        this.onRemoteStreamCallbacks.forEach(cb => cb(stream));
        // Notify about participant stream for group calls
        this.onParticipantStreamCallbacks.forEach(cb => cb(participant.identity, stream));
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

  async acceptCall(callId: string, chatId: string): Promise<void> {
    console.log('[WebRTC] acceptCall called with callId:', callId, 'chatId:', chatId);
    try {
      // Check if LiveKit is available
      if (this.currentCall?.liveKitRoom && this.currentCall?.liveKitURL && this.currentCall?.liveKitToken) {
        console.log('[LiveKit] Using LiveKit for call');
        
        if (!this.localStream) {
          const isVideoCall = this.currentCall?.type === 'video';
          try {
            this.localStream = await navigator.mediaDevices.getUserMedia({
              audio: true,
              video: isVideoCall,
            });
            this.onLocalStreamCallbacks.forEach(cb => cb(this.localStream!));
          } catch (mediaError: any) {
            console.error('[WebRTC] Media access error:', mediaError);
            if (isVideoCall) {
              try {
                this.localStream = await navigator.mediaDevices.getUserMedia({
                  audio: true,
                  video: false,
                });
                if (this.currentCall) {
                  this.currentCall.type = 'audio';
                }
                this.onLocalStreamCallbacks.forEach(cb => cb(this.localStream!));
              } catch (audioError: any) {
                throw new Error('Не удалось получить доступ к микрофону и камере. Проверьте разрешения браузера.');
              }
            } else {
              throw new Error('Не удалось получить доступ к микрофону. Проверьте разрешения браузера.');
            }
          }
        }

        await this.connectToLiveKitRoom(
          this.currentCall.liveKitRoom,
          this.currentCall.liveKitURL,
          this.currentCall.liveKitToken
        );

        wsService.send('call_accept', { call_id: callId });

        if (this.currentCall) {
          this.currentCall.isConnected = true;
          this.notifyStateChange();
        }
        return;
      }

      // Use peer-to-peer WebRTC
      if (!this.pc) {
        console.log('[WebRTC] Creating peer connection');
        this.pc = this.createPeerConnection();
      }

      console.log(`[WebRTC] Signaling state before setup: ${this.pc.signalingState}`);

      if (this.pc.signalingState === 'stable' && this.currentCall?.offerSdp) {
        console.log('[WebRTC] Applying remote offer SDP before accepting');
        await this.pc.setRemoteDescription(new RTCSessionDescription({
          type: 'offer',
          sdp: this.currentCall.offerSdp
        }));
        console.log(`[WebRTC] Signaling state after remote desc: ${this.pc.signalingState}`);
      }

      if (!this.localStream) {
        console.log('[WebRTC] Getting local media stream');
        const isVideoCall = this.currentCall?.type === 'video';
        
        try {
          console.log('[WebRTC] Requesting getUserMedia with video:', isVideoCall);
          this.localStream = await navigator.mediaDevices.getUserMedia({
            audio: true,
            video: isVideoCall,
          });
          console.log('[WebRTC] Got local stream');
        } catch (mediaError: any) {
          console.error('[WebRTC] Media access error:', mediaError);
          if (isVideoCall) {
            console.warn('[WebRTC] Video access failed, falling back to audio-only:', mediaError.message);
            try {
              this.localStream = await navigator.mediaDevices.getUserMedia({
                audio: true,
                video: false,
              });
              if (this.currentCall) {
                this.currentCall.type = 'audio';
              }
            } catch (audioError: any) {
              console.error('[WebRTC] Audio access also failed:', audioError.message);
              throw new Error('Не удалось получить доступ к микрофону и камере. Проверьте разрешения браузера.');
            }
          } else {
            throw new Error('Не удалось получить доступ к микрофону. Проверьте разрешения браузера.');
          }
        }
        
        this.onLocalStreamCallbacks.forEach(cb => cb(this.localStream!));

        this.localStream.getTracks().forEach(track => {
          this.pc!.addTrack(track, this.localStream!);
        });
      }

      console.log('[WebRTC] Sending call_accept signal');
      wsService.send('call_accept', { call_id: callId });

      if (this.currentCall) {
        this.currentCall.isRinging = true;
        this.notifyStateChange();
      }

      console.log('[WebRTC] Creating answer');
      const answer = await this.pc.createAnswer();
      console.log('[WebRTC] Setting local description');
      await this.pc.setLocalDescription(answer);

      console.log('[WebRTC] Sending call_answer with SDP');
      wsService.send('call_answer', {
        call_id: callId,
        sdp: answer.sdp,
      });

      if (this.currentCall) {
        this.currentCall.isRinging = false;
        this.currentCall.isConnected = true;
        this.notifyStateChange();
      }
      console.log('[WebRTC] Call accept completed successfully');
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
    this.remoteStream?.getTracks().forEach(track => track.stop());
    this.remoteStreams.forEach(stream => stream.getTracks().forEach(track => track.stop()));
    this.remoteStreams.clear();

    try {
      if (this.pc) {
        this.pc.close();
      }
      
      if (this.liveKitRoom) {
        this.liveKitRoom.disconnect();
        this.liveKitRoom = null;
      }
    } catch (e) {
      console.error('[WebRTC] Error during cleanup:', e);
    }

    this.localStream = null;
    this.remoteStream = null;
    this.pc = null;
    this.pendingIceCandidates = [];
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
      liveKitRoom: payload.livekit?.room_name,
      liveKitURL: payload.livekit?.url,
      liveKitToken: payload.livekit?.token,
      offerSdp: payload.sdp,
    };

    // If LiveKit info is available, we'll use it in acceptCall instead of peer-to-peer
    if (!payload.livekit) {
      if (!this.pc) {
        this.pc = this.createPeerConnection();
      }
      // We will apply the SDP in acceptCall to ensure it's fully awaited before createAnswer
    }

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

    // Validate required fields
    if (!payload.candidate || payload.sdp_mline_index === undefined || !payload.sdp_mid) {
      console.error('Invalid ICE candidate payload', payload);
      return;
    }

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

export const webCallService = new WebCallService();

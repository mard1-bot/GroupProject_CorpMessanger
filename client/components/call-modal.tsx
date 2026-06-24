import React, { useEffect, useRef, useState } from 'react';
import {
  View,
  Text,
  StyleSheet,
  TouchableOpacity,
  Modal,
  Dimensions,
  ActivityIndicator,
  Platform,
  ScrollView,
  Alert,
} from 'react-native';
import { MaterialIcons } from '@expo/vector-icons';
import { useThemeColor } from '@/hooks/use-theme-color';
import { callService, type CallState, type CallType } from '@/services/calls';
import { RemoteParticipant } from 'livekit-client';
import { wsService } from '@/services/websocket';
import { RTCView } from './rtc-view';

interface CallModalProps {
  visible: boolean;
  onClose: () => void;
  chatId: string;
  calleeId?: string;
  calleeName?: string;
  callType: CallType;
}

export function CallModal({
  visible,
  onClose,
  chatId,
  calleeId,
  calleeName,
  callType,
}: CallModalProps) {
  const [callState, setCallState] = useState<CallState | null>(() => callService.getCurrentCall());
  const [localStream, setLocalStream] = useState<MediaStream | null>(null);
  const [remoteStream, setRemoteStream] = useState<MediaStream | null>(null);
  const [isMuted, setIsMuted] = useState(false);
  const [isVideoOff, setIsVideoOff] = useState(true);
  const [videoFallback, setVideoFallback] = useState(false);
  const [participants, setParticipants] = useState<Map<string, { participant: RemoteParticipant; stream: MediaStream }>>(new Map());

  const [isAccepting, setIsAccepting] = useState(false);

  const localVideoRef = useRef<HTMLVideoElement>(null);
  const remoteVideoRef = useRef<HTMLVideoElement>(null);
  const participantVideoRefs = useRef<Map<string, HTMLVideoElement>>(new Map());

  const primaryColor = useThemeColor({}, 'primary');
  const textColor = useThemeColor({}, 'text');
  const bgColor = useThemeColor({}, 'background');
  const isWeb = Platform.OS === 'web';

  useEffect(() => {
    if (!visible) return;

    const stateUnsub = callService.onStateChange((state: CallState | null) => {
      setCallState(state);
      // Detect video fallback (call type changed from video to audio)
      if (state && callType === 'video' && state.type === 'audio') {
        setVideoFallback(true);
      }
      if (state?.isEnded || !state) {
        setTimeout(onClose, 500);
      }
    });

    const localUnsub = callService.onLocalStream((stream: MediaStream) => {
      setLocalStream(stream);
    });

    const remoteUnsub = callService.onRemoteStream((stream: MediaStream) => {
      setRemoteStream(stream);
    });

    const participantJoinedUnsub = (callService as any).onParticipantJoined?.((participant: RemoteParticipant) => {
      console.log('[CallModal] Participant joined:', participant.identity);
      setParticipants(prev => {
        const newMap = new Map(prev);
        newMap.set(participant.identity, { participant, stream: null as any });
        return newMap;
      });
    });

    const participantLeftUnsub = (callService as any).onParticipantLeft?.((participant: RemoteParticipant) => {
      console.log('[CallModal] Participant left:', participant.identity);
      setParticipants(prev => {
        const newMap = new Map(prev);
        newMap.delete(participant.identity);
        return newMap;
      });
    });

    const participantStreamUnsub = (callService as any).onParticipantStream?.((identity: string, stream: MediaStream) => {
      console.log('[CallModal] Participant stream received:', identity, 'tracks:', stream.getTracks().map(t => t.kind));
      setParticipants(prev => {
        const newMap = new Map(prev);
        const existing = newMap.get(identity);
        if (existing) {
          newMap.set(identity, { ...existing, stream });
        } else {
          // Create placeholder participant if not exists
          newMap.set(identity, { participant: { identity } as RemoteParticipant, stream });
        }
        return newMap;
      });
    });

    return () => {
      stateUnsub();
      localUnsub();
      remoteUnsub();
      participantJoinedUnsub?.();
      participantLeftUnsub?.();
      participantStreamUnsub?.();
    };
  }, [visible, callType, onClose]);

  // Web-specific: set video element srcObject
  useEffect(() => {
    if (isWeb) {
      if (localVideoRef.current && localStream) {
        console.log('[CallModal] Setting local video stream', {
          hasStream: !!localStream,
          tracks: localStream.getTracks().map(t => ({ kind: t.kind, enabled: t.enabled, id: t.id })),
          videoTracks: localStream.getVideoTracks().map(t => ({ id: t.id, enabled: t.enabled, label: t.label }))
        });
        localVideoRef.current.srcObject = localStream;
        localVideoRef.current.muted = true;
        localVideoRef.current.play().catch(err => console.error('Local video play error:', err));
        // Ensure video tracks are enabled
        localStream.getVideoTracks().forEach(track => {
          track.enabled = true;
          console.log('[CallModal] Video track enabled:', track.id, track.enabled, track.label);
        });
      } else {
        console.log('[CallModal] Local video not set:', { hasRef: !!localVideoRef.current, hasStream: !!localStream });
      }

      if (remoteVideoRef.current && remoteStream) {
        console.log('[CallModal] Setting remote video stream', {
          hasStream: !!remoteStream,
          tracks: remoteStream.getTracks().map(t => ({ kind: t.kind, enabled: t.enabled, id: t.id }))
        });
        remoteVideoRef.current.srcObject = remoteStream;
        remoteVideoRef.current.play().catch(err => console.error('Remote video play error:', err));
      } else {
        console.log('[CallModal] Remote video not set:', { hasRef: !!remoteVideoRef.current, hasStream: !!remoteStream });
      }
    }
  }, [localStream, remoteStream, isWeb]);

  // Set participant video streams for group calls
  useEffect(() => {
    if (isWeb) {
      participants.forEach(({ stream }, identity) => {
        if (stream) {
          const videoEl = participantVideoRefs.current.get(identity);
          if (videoEl && videoEl.srcObject !== stream) {
            console.log('[CallModal] Setting participant video stream:', identity);
            videoEl.srcObject = stream;
            videoEl.play().catch(err => console.error('Participant video play error:', identity, err));
          }
        }
      });
    }
  }, [participants, isWeb]);

  const handleEndCall = () => {
    callService.endCall();
    onClose();
  };

  const handleToggleMute = () => {
    const enabled = callService.toggleMute();
    setIsMuted(!enabled);
  };

  const handleToggleVideo = () => {
    const enabled = callService.toggleVideo();
    setIsVideoOff(!enabled);
  };

  const handleAcceptCall = async () => {
    if (!callState) return;
    try {
      setIsAccepting(true);
      if (callState.isGroup) {
        await callService.joinCall(callState.callId, chatId);
      } else {
        await callService.acceptCall(callState.callId, chatId);
      }
    } catch (error) {
      console.error('Failed to accept call:', error);
      setIsAccepting(false);
    }
  };

  const isConnecting = callState?.isCaller && !callState?.isConnected && !callState?.isRinging;
  const isIncoming = callState && !callState.isCaller && !callState.isConnected;
  const isGroupCall = !calleeId && (participants.size > 0 || callState?.liveKitRoom);
  const participantCount = participants.size + (remoteStream ? 1 : 0);

  return (
    <Modal
      visible={visible}
      transparent
      animationType="fade"
      onRequestClose={handleEndCall}
    >
      <View style={[styles.container, { backgroundColor: '#1a1a1a' }]}>
        {/* Header */}
        <View style={styles.header}>
          <Text style={[styles.calleeName, { color: '#fff' }]}>
            {calleeName}
          </Text>
          <Text style={[styles.status, { color: '#888' }]}>
            {videoFallback ? 'Видео недоступно. Аудиозвонок...' :
              isIncoming ? 'Входящий звонок...' :
                isConnecting ? 'Соединение...' :
                  callState?.isRinging ? 'Звонит...' :
                    callState?.isConnected ? 'В разговоре' : 'Звонок завершен'}
          </Text>
        </View>

        {/* Video containers */}
        <View style={styles.videoContainer}>
          {callType === 'video' && !videoFallback && callState?.type === 'video' ? (
            isGroupCall ? (
              // Group call: Grid layout for multiple participants
              <ScrollView contentContainerStyle={styles.videoGrid}>
                {/* Remote stream (if exists and not in participants map) */}
                {remoteStream && !participants.has('remote') && (
                  <View style={styles.gridVideoContainer}>
                    {isWeb ? (
                      <video
                        ref={remoteVideoRef}
                        style={styles.gridVideo}
                        autoPlay
                        playsInline
                        muted={false}
                        controls={false}
                      />
                    ) : (
                      <RTCView
                        streamURL={(remoteStream as any).toURL()}
                        style={styles.gridVideo}
                        objectFit="cover"
                      />
                    )}
                  </View>
                )}
                {/* Participant videos */}
                {Array.from(participants.entries()).map(([identity, { participant, stream }]) => (
                  <View key={identity} style={styles.gridVideoContainer}>
                    {stream ? (
                      isWeb ? (
                        <video
                        ref={(el) => {
                          if (el) {
                            participantVideoRefs.current.set(identity, el);
                          }
                        }}
                          style={styles.gridVideo}
                          autoPlay
                          playsInline
                          muted={false}
                          controls={false}
                        />
                      ) : (
                        <RTCView
                          streamURL={(stream as any).toURL()}
                          style={styles.gridVideo}
                          objectFit="cover"
                        />
                      )
                    ) : (
                      <View style={[styles.gridVideo, styles.noVideo]}>
                        <MaterialIcons name="person" size={40} color="#444" />
                        <Text style={styles.participantName}>{identity}</Text>
                      </View>
                    )}
                  </View>
                ))}
              </ScrollView>
            ) : (
              // 1-on-1 call: Full screen remote video with PIP local video
              <>
                {/* Remote video (full screen) */}
                {remoteStream ? (
                  isWeb ? (
                    <video
                      ref={(el) => {
                        remoteVideoRef.current = el;
                      }}
                      style={styles.remoteVideoWeb}
                      autoPlay
                      playsInline
                      muted={false}
                      controls={false}
                    />
                  ) : (
                    <RTCView
                      streamURL={(remoteStream as any).toURL()}
                      style={styles.remoteVideo}
                      objectFit="cover"
                    />
                  )
                ) : (
                  <View style={[styles.remoteVideo, styles.noVideo]}>
                    <MaterialIcons name="person" size={80} color="#444" />
                  </View>
                )}

                {/* Local video (picture in picture) */}
                {localStream && (
                  <View style={styles.localVideoContainer}>
                    {isWeb ? (
                      <video
                        ref={(el) => {
                          localVideoRef.current = el;
                        }}
                        style={styles.localVideoWeb}
                        autoPlay
                        playsInline
                        muted={true}
                        controls={false}
                      />
                    ) : (
                      <RTCView
                        streamURL={(localStream as any).toURL()}
                        style={styles.localVideo}
                        objectFit="cover"
                        mirror
                      />
                    )}
                    {isVideoOff && (
                      <View style={[styles.localVideo, styles.videoOff]}>
                        <MaterialIcons name="videocam-off" size={24} color="#fff" />
                      </View>
                    )}
                  </View>
                )}
              </>
            )
          ) : (
            <View style={styles.audioContainer}>
              <View style={styles.avatar}>
                <MaterialIcons name="person" size={100} color="#444" />
              </View>
              {isConnecting && (
                <ActivityIndicator size="large" color={primaryColor} style={styles.spinner} />
              )}
            </View>
          )}
        </View>

        {/* Incoming call buttons */}
        {isIncoming ? (
          <View style={styles.incomingButtons}>
            <TouchableOpacity
              style={[styles.button, styles.rejectButton]}
              onPress={handleEndCall}
            >
              <MaterialIcons name="call-end" size={32} color="#fff" />
            </TouchableOpacity>
            <TouchableOpacity
              style={[styles.button, styles.acceptButton, isAccepting && { opacity: 0.7 }]}
              onPress={handleAcceptCall}
              disabled={isAccepting}
            >
              {isAccepting ? (
                <ActivityIndicator color="#fff" size="large" />
              ) : (
                <MaterialIcons name="call" size={32} color="#fff" />
              )}
            </TouchableOpacity>
          </View>
        ) : (
          /* Call control buttons */
          <View style={styles.controls}>
            <TouchableOpacity
              style={[styles.controlButton, isMuted && styles.activeButton]}
              onPress={handleToggleMute}
            >
              <MaterialIcons
                name={isMuted ? 'mic-off' : 'mic'}
                size={28}
                color="#fff"
              />
            </TouchableOpacity>

            {callType === 'video' && (
              <TouchableOpacity
                style={[styles.controlButton, isVideoOff && styles.activeButton]}
                onPress={handleToggleVideo}
              >
                <MaterialIcons
                  name={isVideoOff ? 'videocam-off' : 'videocam'}
                  size={28}
                  color="#fff"
                />
              </TouchableOpacity>
            )}

            <TouchableOpacity
              style={[styles.controlButton, styles.endCallButton]}
              onPress={handleEndCall}
            >
              <MaterialIcons name="call-end" size={28} color="#fff" />
            </TouchableOpacity>
          </View>
        )}
      </View>
    </Modal>
  );
}

const { width, height } = Dimensions.get('window');

const styles = StyleSheet.create({
  container: {
    flex: 1,
    justifyContent: 'space-between',
  },
  header: {
    alignItems: 'center',
    paddingTop: 60,
    paddingHorizontal: 20,
  },
  calleeName: {
    fontSize: 24,
    fontWeight: '600',
    marginBottom: 8,
    color: '#fff',
  },
  status: {
    fontSize: 16,
    color: '#888',
  },
  videoContainer: {
    flex: 1,
    position: 'relative',
  },
  remoteVideo: {
    width: width,
    height: height - 200,
    backgroundColor: '#000',
  },
  remoteVideoWeb: {
    width: width,
    height: height - 200,
    backgroundColor: '#000',
    objectFit: 'cover',
  },
  noVideo: {
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: '#2a2a2a',
  },
  localVideoContainer: {
    position: 'absolute',
    bottom: 200,
    right: 20,
    width: 120,
    height: 160,
    borderRadius: 12,
    overflow: 'hidden',
    borderWidth: 2,
    borderColor: '#fff',
  },
  localVideo: {
    width: 120,
    height: 160,
    backgroundColor: '#000',
  },
  localVideoWeb: {
    width: 120,
    height: 160,
    backgroundColor: '#000',
    objectFit: 'cover',
  },
  videoOff: {
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: '#333',
  },
  audioContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
  },
  avatar: {
    width: 150,
    height: 150,
    borderRadius: 75,
    backgroundColor: '#2a2a2a',
    justifyContent: 'center',
    alignItems: 'center',
  },
  spinner: {
    marginTop: 30,
  },
  controls: {
    flexDirection: 'row',
    justifyContent: 'center',
    alignItems: 'center',
    paddingBottom: 60,
    paddingHorizontal: 40,
    gap: 30,
  },
  controlButton: {
    width: 90,
    height: 90,
    borderRadius: 45,
    backgroundColor: '#333',
    justifyContent: 'center',
    alignItems: 'center',
  },
  activeButton: {
    backgroundColor: '#555',
  },
  endCallButton: {
    backgroundColor: '#e53935',
  },
  incomingButtons: {
    flexDirection: 'row',
    justifyContent: 'center',
    alignItems: 'center',
    paddingBottom: 100,
    gap: 50,
  },
  button: {
    width: 110,
    height: 110,
    borderRadius: 55,
    justifyContent: 'center',
    alignItems: 'center',
  },
  rejectButton: {
    backgroundColor: '#e53935',
  },
  acceptButton: {
    backgroundColor: '#43a047',
  },
  videoGrid: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    justifyContent: 'center',
    padding: 10,
    gap: 10,
  },
  gridVideoContainer: {
    width: (width - 40) / 2,
    height: (height - 300) / 2,
    borderRadius: 12,
    overflow: 'hidden',
    backgroundColor: '#000',
  },
  gridVideo: {
    width: '100%',
    height: '100%',
    backgroundColor: '#000',
  },
  participantName: {
    position: 'absolute',
    bottom: 10,
    left: 10,
    color: '#fff',
    fontSize: 14,
    backgroundColor: 'rgba(0,0,0,0.5)',
    paddingHorizontal: 8,
    paddingVertical: 4,
    borderRadius: 4,
  },
});

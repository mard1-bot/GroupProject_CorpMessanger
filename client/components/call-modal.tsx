import React, { useEffect, useRef, useState } from 'react';
import {
  View,
  Text,
  StyleSheet,
  TouchableOpacity,
  Modal,
  Dimensions,
  ActivityIndicator,
} from 'react-native';
import { MaterialIcons } from '@expo/vector-icons';
import { useThemeColor } from '@/hooks/use-theme-color';
import { callService, type CallState, type CallType } from '@/services/calls';

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
  const [callState, setCallState] = useState<CallState | null>(null);
  const [localStream, setLocalStream] = useState<MediaStream | null>(null);
  const [remoteStream, setRemoteStream] = useState<MediaStream | null>(null);
  const [isMuted, setIsMuted] = useState(false);
  const [isVideoOff, setIsVideoOff] = useState(false);
  
  const localVideoRef = useRef<HTMLVideoElement>(null);
  const remoteVideoRef = useRef<HTMLVideoElement>(null);
  
  const primaryColor = useThemeColor({}, 'primary');
  const textColor = useThemeColor({}, 'text');
  const bgColor = useThemeColor({}, 'background');

  useEffect(() => {
    if (!visible) return;

    const stateUnsub = callService.onStateChange((state) => {
      setCallState(state);
      if (state?.isEnded) {
        setTimeout(onClose, 1500);
      }
    });

    const localUnsub = callService.onLocalStream((stream) => {
      setLocalStream(stream);
    });

    const remoteUnsub = callService.onRemoteStream((stream) => {
      setRemoteStream(stream);
    });

    return () => {
      stateUnsub();
      localUnsub();
      remoteUnsub();
    };
  }, [visible]);

  useEffect(() => {
    if (localVideoRef.current && localStream) {
      localVideoRef.current.srcObject = localStream;
    }
    if (remoteVideoRef.current && remoteStream) {
      remoteVideoRef.current.srcObject = remoteStream;
    }
  }, [localStream, remoteStream]);

  const handleStartCall = async () => {
    if (!calleeId) {
      console.error('No callee ID provided');
      onClose();
      return;
    }
    try {
      await callService.startCall(chatId, calleeId, callType);
    } catch (error) {
      console.error('Failed to start call:', error);
      onClose();
    }
  };

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

  // Start call when modal opens
  const callStartedRef = useRef(false);
  useEffect(() => {
    if (visible && !callState && !callStartedRef.current) {
      callStartedRef.current = true;
      handleStartCall();
    }
    if (!visible) {
      callStartedRef.current = false;
    }
  }, [visible, callState, handleStartCall]);

  const isConnecting = !callState?.isConnected && !callState?.isRinging;
  const isIncoming = callState && !callState.isCaller && !callState.isConnected && !callState.isRinging;

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
            {isIncoming ? 'Входящий звонок...' :
             isConnecting ? 'Соединение...' :
             callState?.isRinging ? 'Звонит...' :
             callState?.isConnected ? 'В разговоре' : 'Звонок завершен'}
          </Text>
        </View>

        {/* Video containers */}
        <View style={styles.videoContainer}>
          {callType === 'video' && (
            <>
              {/* Remote video (full screen) */}
              {remoteStream ? (
                <video
                  ref={remoteVideoRef}
                  style={styles.remoteVideo}
                  autoPlay
                  playsInline
                />
              ) : (
                <View style={[styles.remoteVideo, styles.noVideo]}>
                  <MaterialIcons name="person" size={80} color="#444" />
                </View>
              )}

              {/* Local video (picture in picture) */}
              {localStream && (
                <View style={styles.localVideoContainer}>
                  <video
                    ref={localVideoRef}
                    style={styles.localVideo}
                    autoPlay
                    playsInline
                    muted
                  />
                  {isVideoOff && (
                    <View style={[styles.localVideo, styles.videoOff]}>
                      <MaterialIcons name="videocam-off" size={24} color="#fff" />
                    </View>
                  )}
                </View>
              )}
            </>
          )}

          {callType === 'audio' && (
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
              style={[styles.button, styles.acceptButton]}
              onPress={() => callState && callService.acceptCall(callState.callId, chatId)}
            >
              <MaterialIcons name="call" size={32} color="#fff" />
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
  },
  status: {
    fontSize: 16,
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
    width: 60,
    height: 60,
    borderRadius: 30,
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
    width: 80,
    height: 80,
    borderRadius: 40,
    justifyContent: 'center',
    alignItems: 'center',
  },
  rejectButton: {
    backgroundColor: '#e53935',
  },
  acceptButton: {
    backgroundColor: '#43a047',
  },
});

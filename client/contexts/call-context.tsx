import React, { createContext, useContext, useEffect, useState, useCallback } from 'react';
import { callService, type CallState } from '@/services/calls';
import { CallModal } from '@/components/call-modal';
import { api } from '@/services/api';

interface CallContextType {
  currentCall: CallState | null;
  isInCall: boolean;
}

const CallContext = createContext<CallContextType>({
  currentCall: null,
  isInCall: false,
});

export function useCall() {
  return useContext(CallContext);
}

interface CallProviderProps {
  children: React.ReactNode;
}

export function CallProvider({ children }: CallProviderProps) {
  const [currentCall, setCurrentCall] = useState<CallState | null>(null);
  const [displayName, setDisplayName] = useState<string>('Пользователь');
  const [isGroupCall, setIsGroupCall] = useState(false);
  const [showIncomingCall, setShowIncomingCall] = useState(false);
  const [nameResolved, setNameResolved] = useState(false);

  useEffect(() => {
    const unsubscribe = callService.onStateChange((state: CallState | null) => {
      setCurrentCall(state);
      
      if (!state) {
        setShowIncomingCall(false);
        setNameResolved(false);
        setDisplayName('Пользователь');
        return;
      }
      
      // Resolve the name of the other participant
      if (!nameResolved) {
        // For caller: resolve callee's name; for callee: resolve caller's name
        const otherUserId = state.isCaller ? state.calleeId : state.callerId;
        
        if (otherUserId) {
          api.getUserById(otherUserId).then(res => {
            if (res.data) {
              const fullName = `${res.data.first_name} ${res.data.last_name}`;
              // Check if it's a group call
              if (state.chatId) {
                api.getChatById(state.chatId).then(chatRes => {
                  if (chatRes.data && chatRes.data.type === 'group') {
                    setIsGroupCall(true);
                    setDisplayName(chatRes.data.title || fullName);
                  } else {
                    setIsGroupCall(false);
                    setDisplayName(fullName);
                  }
                  setNameResolved(true);
                }).catch(() => {
                  setIsGroupCall(false);
                  setDisplayName(fullName);
                  setNameResolved(true);
                });
              } else {
                setDisplayName(fullName);
                setNameResolved(true);
              }
            }
          }).catch(() => {
            setNameResolved(true);
          });
        } else {
          // No other user ID available (e.g. group call without callee)
          if (state.chatId) {
            api.getChatById(state.chatId).then(chatRes => {
              if (chatRes.data && chatRes.data.type === 'group') {
                setIsGroupCall(true);
                setDisplayName(chatRes.data.title || 'Групповой звонок');
              }
              setNameResolved(true);
            }).catch(() => {
              setNameResolved(true);
            });
          } else {
            setNameResolved(true);
          }
        }
      }
      
      // Show incoming call UI for non-caller
      if (!state.isCaller && !state.isConnected && !state.isRinging) {
        setShowIncomingCall(true);
      }
      
      // Hide incoming call modal when call is connected or ended
      if (state.isConnected || state.isEnded) {
        setShowIncomingCall(false);
      }
    });

    return () => {
      unsubscribe();
    };
  }, [nameResolved]);

  const handleAcceptCall = useCallback(async () => {
    console.log('Accept call pressed, currentCall:', currentCall);
    if (currentCall) {
      try {
        console.log('Calling acceptCall with callId:', currentCall.callId, 'chatId:', currentCall.chatId);
        await callService.acceptCall(currentCall.callId, currentCall.chatId);
        setShowIncomingCall(false);
        console.log('Call accepted successfully');
      } catch (error) {
        console.error('Failed to accept call:', error);
      }
    } else {
      console.error('No current call to accept');
    }
  }, [currentCall]);

  const handleRejectCall = useCallback(() => {
    if (currentCall) {
      callService.rejectCall(currentCall.callId);
    }
    setShowIncomingCall(false);
  }, [currentCall]);

  const value = {
    currentCall,
    isInCall: !!currentCall && !currentCall.isEnded,
  };

  return (
    <CallContext.Provider value={value}>
      {children}
      <CallModal
        visible={!!currentCall}
        onClose={() => {
          if (currentCall && !currentCall.isEnded) {
            callService.endCall();
          }
          setShowIncomingCall(false);
        }}
        chatId={currentCall?.chatId || ''}
        calleeId={(currentCall?.isCaller ? currentCall.calleeId : currentCall?.callerId) || undefined}
        calleeName={displayName}
        callType={currentCall?.type || 'audio'}
      />
    </CallContext.Provider>
  );
}

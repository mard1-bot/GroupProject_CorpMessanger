import React, { createContext, useContext, useEffect, useState, useCallback } from 'react';
import { callService, type CallState } from '@/services/calls';
import { IncomingCall } from '@/components/incoming-call';
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
  const [callerName, setCallerName] = useState<string>('Пользователь');
  const [isGroupCall, setIsGroupCall] = useState(false);
  const [showIncomingCall, setShowIncomingCall] = useState(false);

  useEffect(() => {
    const unsubscribe = callService.onStateChange((state) => {
      setCurrentCall(state);
      
      // Show incoming call modal when receiving a call
      if (state && !state.isCaller && !state.isConnected && !state.isRinging) {
        setShowIncomingCall(true);
        // Fetch caller info and chat info for group name
        api.getUserById(state.callerId).then(res => {
          if (res.data) {
            const callerFullName = `${res.data.first_name} ${res.data.last_name}`;
            // Also fetch chat info to check if it's a group call
            if (state.chatId) {
              api.getChatById(state.chatId).then(chatRes => {
                if (chatRes.data && chatRes.data.type === 'group') {
                  setIsGroupCall(true);
                  setCallerName(chatRes.data.title || callerFullName);
                } else {
                  setIsGroupCall(false);
                  setCallerName(callerFullName);
                }
              }).catch(() => {
                setIsGroupCall(false);
                setCallerName(callerFullName);
              });
            } else {
              setCallerName(callerFullName);
            }
          }
        });
      }
      
      // Hide incoming call modal when call is connected or ended
      if (!state || state.isConnected || state.isEnded) {
        setShowIncomingCall(false);
      }
    });

    return () => {
      unsubscribe();
    };
  }, []);

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
      {showIncomingCall && currentCall && (
        <IncomingCall
          callState={currentCall}
          callerName={callerName}
          isGroup={isGroupCall}
          onAccept={handleAcceptCall}
          onReject={handleRejectCall}
        />
      )}
    </CallContext.Provider>
  );
}

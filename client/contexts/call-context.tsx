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
  const [showIncomingCall, setShowIncomingCall] = useState(false);

  useEffect(() => {
    const unsubscribe = callService.onStateChange((state) => {
      setCurrentCall(state);
      
      // Show incoming call modal when receiving a call
      if (state && !state.isCaller && !state.isConnected && !state.isRinging) {
        setShowIncomingCall(true);
        // Fetch caller info
        api.getUserById(state.callerId).then(res => {
          if (res.data) {
            setCallerName(`${res.data.first_name} ${res.data.last_name}`);
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

  const handleAcceptCall = useCallback(() => {
    if (currentCall) {
      callService.acceptCall(currentCall.callId, currentCall.chatId);
    }
    setShowIncomingCall(false);
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
          onAccept={handleAcceptCall}
          onReject={handleRejectCall}
        />
      )}
    </CallContext.Provider>
  );
}

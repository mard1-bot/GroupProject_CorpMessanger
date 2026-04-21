import { useState, useCallback, useRef, useEffect } from 'react';
import { wsService, WSTypingPayload } from '@/services/websocket';

interface TypingUser {
  user_id: string;
  first_name?: string;
  last_name?: string;
}

export function useTyping(chatId: string | undefined, currentUserId: string | undefined, firstName?: string, lastName?: string) {
  const [typingUsers, setTypingUsers] = useState<TypingUser[]>([]);
  const typingTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const isTypingRef = useRef(false);

  // Listen for typing events
  useEffect(() => {
    if (!chatId) return;

    const unsubscribe = wsService.onMessage((message) => {
      if (message.type === 'typing' && message.chat_id === chatId) {
        const payload = message.payload as WSTypingPayload;
        
        // Don't show current user typing
        if (payload.user_id === currentUserId) return;

        setTypingUsers((prev) => {
          if (payload.is_typing) {
            // Add or update user
            const filtered = prev.filter(u => u.user_id !== payload.user_id);
            return [...filtered, {
              user_id: payload.user_id,
              first_name: payload.first_name,
              last_name: payload.last_name,
            }];
          } else {
            // Remove user
            return prev.filter(u => u.user_id !== payload.user_id);
          }
        });
      }
    });

    return () => {
      unsubscribe();
      // Clear typing on unmount
      if (isTypingRef.current) {
        wsService.sendTyping(chatId, false, firstName, lastName);
      }
    };
  }, [chatId, currentUserId, firstName, lastName]);

  // Clear typing users after timeout (5 seconds)
  useEffect(() => {
    if (typingUsers.length === 0) return;

    const interval = setInterval(() => {
      // In real implementation, server would broadcast stop typing
      // Here we just clear after some time as fallback
      setTypingUsers((prev) => {
        if (prev.length > 0 && Math.random() < 0.1) { // Random clear for demo
          return prev.slice(0, -1);
        }
        return prev;
      });
    }, 5000);

    return () => clearInterval(interval);
  }, [typingUsers]);

  const sendTypingStart = useCallback(() => {
    if (!chatId || isTypingRef.current) return;
    
    isTypingRef.current = true;
    wsService.sendTyping(chatId, true, firstName, lastName);

    // Auto-stop after 5 seconds of inactivity
    if (typingTimeoutRef.current) {
      clearTimeout(typingTimeoutRef.current);
    }
    typingTimeoutRef.current = setTimeout(() => {
      sendTypingStop();
    }, 5000);
  }, [chatId, firstName, lastName]);

  const sendTypingStop = useCallback(() => {
    if (!chatId || !isTypingRef.current) return;
    
    isTypingRef.current = false;
    wsService.sendTyping(chatId, false, firstName, lastName);

    if (typingTimeoutRef.current) {
      clearTimeout(typingTimeoutRef.current);
      typingTimeoutRef.current = null;
    }
  }, [chatId, firstName, lastName]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (typingTimeoutRef.current) {
        clearTimeout(typingTimeoutRef.current);
      }
    };
  }, []);

  const getTypingText = useCallback(() => {
    if (typingUsers.length === 0) return null;
    
    if (typingUsers.length === 1) {
      const user = typingUsers[0];
      const name = user.first_name || 'Кто-то';
      return `${name} печатает...`;
    } else if (typingUsers.length === 2) {
      const name1 = typingUsers[0].first_name || 'Кто-то';
      const name2 = typingUsers[1].first_name || 'Кто-то';
      return `${name1} и ${name2} печатают...`;
    } else {
      return `${typingUsers.length} человек печатают...`;
    }
  }, [typingUsers]);

  return {
    typingUsers,
    typingText: getTypingText(),
    sendTypingStart,
    sendTypingStop,
  };
}

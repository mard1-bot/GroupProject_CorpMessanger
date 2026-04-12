import { useState, useCallback } from 'react';
import api, { ApiResponse, ApiError } from '@/services/api';

interface UseApiState<T> {
  data: T | null;
  loading: boolean;
  error: ApiError | null;
}

type ApiFunction<T, Args extends unknown[]> = (...args: Args) => Promise<ApiResponse<T>>;

export function useApi<T, Args extends unknown[]>(apiFunc: ApiFunction<T, Args>) {
  const [state, setState] = useState<UseApiState<T>>({
    data: null,
    loading: false,
    error: null,
  });

  const execute = useCallback(async (...args: Args): Promise<ApiResponse<T>> => {
    setState(prev => ({ ...prev, loading: true, error: null }));

    try {
      const response = await apiFunc(...args);

      if (response.error) {
        setState({
          data: null,
          loading: false,
          error: response.error,
        });
      } else {
        setState({
          data: response.data || null,
          loading: false,
          error: null,
        });
      }

      return response;
    } catch (err) {
      const error: ApiError = {
        code: 'unknown_error',
        message: err instanceof Error ? err.message : 'Unknown error occurred',
      };
      setState({
        data: null,
        loading: false,
        error,
      });
      return { error };
    }
  }, [apiFunc]);

  const reset = useCallback(() => {
    setState({
      data: null,
      loading: false,
      error: null,
    });
  }, []);

  return {
    ...state,
    execute,
    reset,
  };
}

// Predefined hooks for common API operations
export function useLogin() {
  return useApi(api.login.bind(api));
}

export function useRegister() {
  return useApi(api.register.bind(api));
}

export function useLogout() {
  return useApi(api.logout.bind(api));
}

export function useCurrentUser() {
  return useApi(api.getCurrentUser.bind(api));
}

export function useUsers() {
  return useApi(api.getUsers.bind(api));
}

export function useChats() {
  return useApi(api.getUserChats.bind(api));
}

export function useCreateChat() {
  return useApi(api.createChat.bind(api));
}

export function useChatMessages() {
  return useApi(api.getChatMessages.bind(api));
}

export function useSendMessage() {
  return useApi(api.sendMessage.bind(api));
}

export function useHealth() {
  return useApi(api.health.bind(api));
}

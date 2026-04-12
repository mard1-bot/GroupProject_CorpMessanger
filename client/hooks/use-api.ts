import { useState, useCallback, useMemo } from 'react';
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
// Using useMemo to create stable function references and prevent infinite re-renders
export function useLogin() {
  const apiFunc = useMemo(() => api.login.bind(api), []);
  return useApi(apiFunc);
}

export function useRegister() {
  const apiFunc = useMemo(() => api.register.bind(api), []);
  return useApi(apiFunc);
}

export function useLogout() {
  const apiFunc = useMemo(() => api.logout.bind(api), []);
  return useApi(apiFunc);
}

export function useCurrentUser() {
  const apiFunc = useMemo(() => api.getCurrentUser.bind(api), []);
  return useApi(apiFunc);
}

export function useUsers() {
  const apiFunc = useMemo(() => api.getUsers.bind(api), []);
  return useApi(apiFunc);
}

export function useChats() {
  const apiFunc = useMemo(() => api.getUserChats.bind(api), []);
  return useApi(apiFunc);
}

export function useCreateChat() {
  const apiFunc = useMemo(() => api.createChat.bind(api), []);
  return useApi(apiFunc);
}

export function useChatMessages() {
  const apiFunc = useMemo(() => api.getChatMessages.bind(api), []);
  return useApi(apiFunc);
}

export function useSendMessage() {
  const apiFunc = useMemo(() => api.sendMessage.bind(api), []);
  return useApi(apiFunc);
}

export function useHealth() {
  const apiFunc = useMemo(() => api.health.bind(api), []);
  return useApi(apiFunc);
}

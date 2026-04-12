import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react';
import api, { User, AuthResponse } from '@/services/api';
import AsyncStorage from '@react-native-async-storage/async-storage';

interface AuthContextType {
  user: User | null;
  token: string | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  login: (email: string, password: string) => Promise<boolean>;
  register: (data: {
    email: string;
    password: string;
    first_name: string;
    last_name: string;
    middle_name?: string;
    phone?: string;
  }) => Promise<boolean>;
  logout: () => Promise<void>;
  refreshUser: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

const AUTH_TOKEN_KEY = '@auth_token';
const USER_DATA_KEY = '@user_data';

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Load auth state from storage on mount
  useEffect(() => {
    loadAuthState();
  }, []);

  const loadAuthState = async () => {
    try {
      const storedToken = await AsyncStorage.getItem(AUTH_TOKEN_KEY);
      const storedUser = await AsyncStorage.getItem(USER_DATA_KEY);

      if (storedToken && storedUser) {
        api.setToken(storedToken);
        setToken(storedToken);
        setUser(JSON.parse(storedUser));
      }
    } catch (error) {
      console.error('Failed to load auth state:', error);
    } finally {
      setIsLoading(false);
    }
  };

  const saveAuthState = async (authData: AuthResponse) => {
    try {
      await AsyncStorage.setItem(AUTH_TOKEN_KEY, authData.token);
      await AsyncStorage.setItem(USER_DATA_KEY, JSON.stringify(authData.user));
      api.setToken(authData.token);
      setToken(authData.token);
      setUser(authData.user);
    } catch (error) {
      console.error('Failed to save auth state:', error);
    }
  };

  const clearAuthState = async () => {
    try {
      await AsyncStorage.removeItem(AUTH_TOKEN_KEY);
      await AsyncStorage.removeItem(USER_DATA_KEY);
      api.setToken(null);
      setToken(null);
      setUser(null);
    } catch (error) {
      console.error('Failed to clear auth state:', error);
    }
  };

  const login = async (email: string, password: string): Promise<boolean> => {
    const response = await api.login({ email, password });

    if (response.error || !response.data) {
      return false;
    }

    await saveAuthState(response.data);
    return true;
  };

  const register = async (data: {
    email: string;
    password: string;
    first_name: string;
    last_name: string;
    middle_name?: string;
    phone?: string;
  }): Promise<boolean> => {
    const response = await api.register(data);

    if (response.error || !response.data) {
      return false;
    }

    await saveAuthState(response.data);
    return true;
  };

  const logout = async (): Promise<void> => {
    await api.logout();
    await clearAuthState();
  };

  const refreshUser = async (): Promise<void> => {
    if (!token) return;

    const response = await api.getCurrentUser();
    if (response.data) {
      setUser(response.data);
      await AsyncStorage.setItem(USER_DATA_KEY, JSON.stringify(response.data));
    }
  };

  const value: AuthContextType = {
    user,
    token,
    isLoading,
    isAuthenticated: !!token && !!user,
    login,
    register,
    logout,
    refreshUser,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}

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

// Check if running in web environment
const isWeb = typeof window !== 'undefined' && typeof window.localStorage !== 'undefined';

// Web storage wrapper using localStorage
const storage = {
  getItem: async (key: string): Promise<string | null> => {
    if (isWeb) {
      return localStorage.getItem(key);
    }
    return AsyncStorage.getItem(key);
  },
  setItem: async (key: string, value: string): Promise<void> => {
    if (isWeb) {
      localStorage.setItem(key, value);
    } else {
      await AsyncStorage.setItem(key, value);
    }
  },
  removeItem: async (key: string): Promise<void> => {
    if (isWeb) {
      localStorage.removeItem(key);
    } else {
      await AsyncStorage.removeItem(key);
    }
  }
};

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
      console.log('Loading auth state...');
      console.log('localStorage keys:', isWeb ? Object.keys(localStorage) : 'N/A');
      const storedToken = await storage.getItem(AUTH_TOKEN_KEY);
      const storedUser = await storage.getItem(USER_DATA_KEY);
      console.log('Stored token:', storedToken ? 'found' : 'not found');
      console.log('Stored user:', storedUser ? 'found' : 'not found');

      if (storedToken && storedUser) {
        let parsedUser: User;
        try {
          parsedUser = JSON.parse(storedUser);
        } catch (parseError) {
          console.error('Failed to parse stored user data:', parseError);
          await storage.removeItem(USER_DATA_KEY);
          setIsLoading(false);
          return;
        }
        api.setToken(storedToken);
        setToken(storedToken);
        setUser(parsedUser);
        console.log('Auth state restored successfully');
      } else {
        console.log('No stored auth state found');
      }
    } catch (error) {
      console.error('Failed to load auth state:', error);
    } finally {
      setIsLoading(false);
    }
  };

  const saveAuthState = async (authData: AuthResponse) => {
    try {
      console.log('Saving auth state, token:', authData.token ? 'exists' : 'missing');
      console.log('isWeb:', isWeb);
      await storage.setItem(AUTH_TOKEN_KEY, authData.token);
      await storage.setItem(USER_DATA_KEY, JSON.stringify(authData.user));
      console.log('localStorage after save:', isWeb ? Object.keys(localStorage) : 'N/A');
      api.setToken(authData.token);
      setToken(authData.token);
      setUser(authData.user);
    } catch (error) {
      console.error('Failed to save auth state:', error);
    }
  };

  const clearAuthState = async () => {
    try {
      await storage.removeItem(AUTH_TOKEN_KEY);
      await storage.removeItem(USER_DATA_KEY);
      api.setToken(null);
      setToken(null);
      setUser(null);
    } catch (error) {
      console.error('Failed to clear auth state:', error);
    }
  };

  const login = async (email: string, password: string): Promise<boolean> => {
    const response = await api.login({ email, password });
    console.log('Login response:', response);
    if (response.error || !response.data) {
      console.error('Login failed:', response.error);
      throw new Error(response.error?.message || 'Login failed');
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
      await storage.setItem(USER_DATA_KEY, JSON.stringify(response.data));
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

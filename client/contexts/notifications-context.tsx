import { createContext, useContext, useEffect, useState, ReactNode } from 'react';
import * as Notifications from 'expo-notifications';
import * as Device from 'expo-device';
import Constants from 'expo-constants';
import { Platform } from 'react-native';
import { api } from '@/services/api';

Notifications.setNotificationHandler({
  handleNotification: async () => ({
    shouldShowAlert: true,
    shouldPlaySound: true,
    shouldSetBadge: true,
    shouldShowBanner: true,
    shouldShowList: true,
  } as any),
});

interface NotificationContextType {
  expoPushToken: string | null;
  notification: Notifications.Notification | null;
  unreadCount: number;
  setBadgeCount: (count: number) => void;
  refreshUnreadCount: () => Promise<void>;
}

const NotificationContext = createContext<NotificationContextType | null>(null);

export function NotificationProvider({ children }: { children: ReactNode }) {
  const [expoPushToken, setExpoPushToken] = useState<string | null>(null);
  const [notification, setNotification] = useState<Notifications.Notification | null>(null);
  const [unreadCount, setUnreadCount] = useState(0);

  // Register for push notifications
  useEffect(() => {
    registerForPushNotificationsAsync().then(token => {
      if (token) {
        setExpoPushToken(token);
      }
    });

    // Listen for incoming notifications (foreground/background)
    const notificationListener = Notifications.addNotificationReceivedListener((notification: Notifications.Notification) => {
      setNotification(notification);
      // Increment badge
      setUnreadCount(prev => prev + 1);
      Notifications.setBadgeCountAsync(unreadCount + 1);
    });

    // Listen for notification response (user tapped notification)
    const responseListener = Notifications.addNotificationResponseReceivedListener((response: Notifications.NotificationResponse) => {
      const data = response.notification.request.content.data as { chat_id?: string };
      // Navigate to chat if chat_id present
      if (data?.chat_id) {
        // Navigation will be handled by the app
        console.log('Navigate to chat:', data.chat_id);
      }
    });

    return () => {
      notificationListener.remove();
      responseListener.remove();
    };
  }, []);

  // Load initial unread count
  useEffect(() => {
    refreshUnreadCount();
  }, []);

  // Register device token with backend
  useEffect(() => {
    if (expoPushToken) {
      api.registerDeviceToken({
        token: expoPushToken,
        platform: Platform.OS as 'ios' | 'android',
        device_name: Device.modelName || undefined,
      }).catch(console.error);
    }
  }, [expoPushToken]);

  async function registerWebPush(): Promise<string | null> {
    if (!('serviceWorker' in navigator) || !('PushManager' in window)) {
      console.log('Web Push is not supported in this browser');
      return null;
    }

    try {
      const permission = await Notification.requestPermission();
      if (permission !== 'granted') {
        console.log('Web Push permission denied');
        return null;
      }

      const registration = await navigator.serviceWorker.register('/sw.js');
      await navigator.serviceWorker.ready;

      // Base64 to Uint8Array converter
      const urlBase64ToUint8Array = (base64String: string) => {
        const padding = '='.repeat((4 - base64String.length % 4) % 4);
        const base64 = (base64String + padding).replace(/\-/g, '+').replace(/_/g, '/');
        const rawData = window.atob(base64);
        const outputArray = new Uint8Array(rawData.length);
        for (let i = 0; i < rawData.length; ++i) {
          outputArray[i] = rawData.charCodeAt(i);
        }
        return outputArray;
      };

      const vapidPublicKey = 'BDxQrS09IYxG9Za7P2VJ4mRBi1yASey-8PqskldZypnoL_Rqu6EMMy1n7UWxXAjrpyQxzMuGtoj_7MONOnQUVXw';
      
      const subscription = await registration.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToUint8Array(vapidPublicKey),
      });

      const subJSON = subscription.toJSON();
      
      // Register with backend
      if (subJSON.endpoint && subJSON.keys) {
        await api.registerWebPushSubscription({
          endpoint: subJSON.endpoint,
          key: subJSON.keys.p256dh,
          auth: subJSON.keys.auth,
        });
        console.log('Web Push subscription registered successfully');
      }

      return 'web-push-registered';
    } catch (error) {
      console.error('Error registering web push:', error);
      return null;
    }
  }

  async function registerForPushNotificationsAsync(): Promise<string | null> {
    if (Platform.OS === 'web') {
      return await registerWebPush();
    }

    if (!Device.isDevice) {
      console.log('Must use physical device for push notifications');
      return null;
    }

    const { status: existingStatus } = await Notifications.getPermissionsAsync();
    let finalStatus = existingStatus;

    if (existingStatus !== 'granted') {
      const { status } = await Notifications.requestPermissionsAsync();
      finalStatus = status;
    }

    if (finalStatus !== 'granted') {
      console.log('Failed to get push token for push notification!');
      return null;
    }

    const projectId = Constants.expoConfig?.extra?.eas?.projectId || 'messenger-app';
    const token = await Notifications.getExpoPushTokenAsync({
      projectId,
    });
    return token.data;
  }

  async function refreshUnreadCount() {
    try {
      const response = await api.getUnreadCount();
      if (response.data) {
        setUnreadCount(response.data.total_unread);
        Notifications.setBadgeCountAsync(response.data.total_unread);
      }
    } catch (error) {
      console.error('Failed to get unread count:', error);
    }
  }

  function setBadgeCount(count: number) {
    setUnreadCount(count);
    Notifications.setBadgeCountAsync(count);
  }

  return (
    <NotificationContext.Provider
      value={{
        expoPushToken,
        notification,
        unreadCount,
        setBadgeCount,
        refreshUnreadCount,
      }}
    >
      {children}
    </NotificationContext.Provider>
  );
}

export function useNotifications() {
  const context = useContext(NotificationContext);
  if (!context) {
    throw new Error('useNotifications must be used within NotificationProvider');
  }
  return context;
}

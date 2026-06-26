import * as Notifications from 'expo-notifications';
import { Platform } from 'react-native';
import Constants from 'expo-constants';
import { api } from './api';
import { useAuth } from '@/contexts/auth-context';

// Configure notification handler
Notifications.setNotificationHandler({
  handleNotification: async () => ({
    shouldShowAlert: true,
    shouldPlaySound: true,
    shouldSetBadge: true,
    shouldShowBanner: true,
    shouldShowList: true,
  }),
});

class NotificationService {
  private token: string | null = null;
  private isRegistered: boolean = false;

  async requestPermissions(): Promise<boolean> {
    try {
      if (Platform.OS === 'android') {
        await Notifications.setNotificationChannelAsync('messages', {
          name: 'Messages',
          importance: Notifications.AndroidImportance.HIGH,
          vibrationPattern: [0, 250, 250, 250],
          lightColor: '#FF231F7C',
        });
      }

      const { status: existingStatus } = await Notifications.getPermissionsAsync();
      let finalStatus = existingStatus;
      
      if (existingStatus !== 'granted') {
        const { status } = await Notifications.requestPermissionsAsync();
        finalStatus = status;
      }

      return finalStatus === 'granted';
    } catch (error) {
      console.error('Failed to request notification permissions:', error);
      return false;
    }
  }

  async registerWebPush(): Promise<void> {
    console.log('[Web Push] Starting registration process');
    if (!('serviceWorker' in navigator) || !('PushManager' in window)) {
      console.log('[Web Push] Error: Not supported in this browser');
      return;
    }

    try {
      console.log('[Web Push] Requesting permission...');
      const permission = await Notification.requestPermission();
      console.log('[Web Push] Permission result:', permission);
      
      if (permission !== 'granted') {
        console.log('[Web Push] Error: Permission denied');
        return;
      }

      console.log('[Web Push] Registering service worker /sw.js');
      const registration = await navigator.serviceWorker.register('/sw.js');
      console.log('[Web Push] Waiting for service worker to be ready...');
      await navigator.serviceWorker.ready;
      console.log('[Web Push] Service worker ready');

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
      
      console.log('[Web Push] Checking for existing subscription...');
      let subscription = await registration.pushManager.getSubscription();
      
      if (subscription) {
        console.log('[Web Push] Unsubscribing from old subscription...');
        await subscription.unsubscribe();
      }

      console.log('[Web Push] Subscribing to push manager with new key...');
      subscription = await registration.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToUint8Array(vapidPublicKey),
      });
      console.log('[Web Push] Push manager subscribed');

      const subJSON = subscription.toJSON();
      console.log('[Web Push] Subscription JSON:', subJSON);
      
      if (subJSON.endpoint && subJSON.keys) {
        console.log('[Web Push] Sending subscription to backend...');
        await api.registerWebPushSubscription({
          endpoint: subJSON.endpoint,
          key: subJSON.keys.p256dh,
          auth: subJSON.keys.auth,
        });
        console.log('[Web Push] Web Push subscription registered successfully');
      } else {
        console.log('[Web Push] Error: Invalid subscription JSON structure');
      }
    } catch (error) {
      console.error('[Web Push] Error during registration:', error);
    }
  }

  async registerDeviceToken(): Promise<void> {
    try {
      if (Platform.OS === 'web') {
        await this.registerWebPush();
        return;
      }

      const hasPermission = await this.requestPermissions();
      if (!hasPermission) {
        console.warn('Notification permissions not granted');
        return;
      }

      const projectId = Constants.expoConfig?.extra?.eas?.projectId || 'messenger-app';
      const token = await Notifications.getExpoPushTokenAsync({
        projectId,
      });

      this.token = token.data;

      if (this.token && !this.isRegistered) {
        await api.registerDeviceToken({
          token: this.token,
          platform: Platform.OS === 'ios' ? 'ios' : 'android',
          device_name: Platform.OS,
        });
        this.isRegistered = true;
        console.log('Device token registered successfully');
      }
    } catch (error) {
      console.error('Failed to register device token:', error);
    }
  }

  setupNotificationListeners(): (() => void) {
    // Handle notification received while app is in foreground
    const subscription = Notifications.addNotificationReceivedListener((notification) => {
      console.log('Notification received:', notification);
    });

    // Handle notification tapped while app is in background
    const responseSubscription = Notifications.addNotificationResponseReceivedListener((response) => {
      console.log('Notification tapped:', response);
      // Navigate to the relevant chat/message
    });

    return () => {
      subscription.remove();
      responseSubscription.remove();
    };
  }

  getToken(): string | null {
    return this.token;
  }

  clearRegistration(): void {
    this.token = null;
    this.isRegistered = false;
  }
}

export const notificationService = new NotificationService();

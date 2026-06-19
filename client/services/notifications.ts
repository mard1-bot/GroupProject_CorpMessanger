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

  async registerDeviceToken(): Promise<void> {
    try {
      const hasPermission = await this.requestPermissions();
      if (!hasPermission) {
        console.warn('Notification permissions not granted');
        return;
      }

      const projectId = Constants.expoConfig?.extra?.eas?.projectId || 'messenger-app';
      const token = await Notifications.getExpoPushTokenAsync({
        projectId,
        vapidPublicKey: 'BNDRILYKeziLxERhD-fevCbrdKnJ7rTlBuvnImHQUg-d9jLYIJkMKXESxiXUY3ykWIbgEKgv6kO6qiZ17xyXoMo'
      });

      this.token = token.data;

      if (this.token && !this.isRegistered) {
        await api.registerDeviceToken({
          token: this.token,
          platform: Platform.OS === 'web' ? 'web' : Platform.OS === 'ios' ? 'ios' : 'android',
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

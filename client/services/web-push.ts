import { api } from './api';

interface WebPushSubscription {
  id: string;
  user_id: string;
  endpoint: string;
  key: string;
  auth: string;
  created_at: string;
}

class WebPushService {
  private isSupported: boolean = false;
  private registration: ServiceWorkerRegistration | null = null;
  private subscription: PushSubscription | null = null;

  constructor() {
    if (typeof window !== 'undefined' && 'serviceWorker' in navigator && 'PushManager' in window) {
      this.isSupported = true;
    }
  }

  async registerServiceWorker(): Promise<void> {
    if (!this.isSupported) {
      console.warn('[WebPush] Push notifications are not supported');
      return;
    }

    try {
      this.registration = await navigator.serviceWorker.register('/sw.js');
      console.log('[WebPush] Service Worker registered', this.registration);
    } catch (error) {
      console.error('[WebPush] Service Worker registration failed', error);
    }
  }

  async requestPermission(): Promise<boolean> {
    if (!this.isSupported) {
      return false;
    }

    const permission = await Notification.requestPermission();
    return permission === 'granted';
  }

  async subscribeUser(): Promise<void> {
    if (!this.registration) {
      console.warn('[WebPush] Service Worker not registered');
      return;
    }

    try {
      // Convert VAPID public key from base64 to Uint8Array
      const vapidPublicKey = 'BH_xzs0hCskJdrMdi84_cm24zeWzyCSskp9Kp5NKyuTg1gq3k4nIIaKq_cGxNyDfnHenKdCiwreO2hd2Tkdirzo';
      const convertedVapidKey = this.urlBase64ToUint8Array(vapidPublicKey) as BufferSource;

      this.subscription = await this.registration.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: convertedVapidKey,
      });

      console.log('[WebPush] Push subscription created', this.subscription);

      // Send subscription to backend
      await this.sendSubscriptionToBackend(this.subscription);
    } catch (error) {
      console.error('[WebPush] Failed to subscribe to push', error);
    }
  }

  async unsubscribeUser(): Promise<void> {
    if (this.subscription) {
      try {
        await this.subscription.unsubscribe();
        console.log('[WebPush] Unsubscribed from push');
      } catch (error) {
        console.error('[WebPush] Failed to unsubscribe', error);
      }
      this.subscription = null;
    }
  }

  private async sendSubscriptionToBackend(subscription: PushSubscription): Promise<void> {
    const subscriptionData = {
      endpoint: subscription.endpoint,
      key: this.getKey(subscription, 'p256dh'),
      auth: this.getKey(subscription, 'auth'),
    };

    try {
      await api.registerWebPushSubscription(subscriptionData);
      console.log('[WebPush] Subscription sent to backend');
    } catch (error) {
      console.error('[WebPush] Failed to send subscription to backend', error);
    }
  }

  private getKey(subscription: PushSubscription, keyType: 'p256dh' | 'auth'): string {
    const key = subscription.getKey(keyType);
    if (!key) return '';
    const keyArray = new Uint8Array(key);
    let keyString = '';
    for (let i = 0; i < keyArray.length; i++) {
      keyString += String.fromCharCode(keyArray[i]);
    }
    return btoa(keyString);
  }

  private urlBase64ToUint8Array(base64String: string): Uint8Array {
    const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
    const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
    const rawData = window.atob(base64);
    const outputArray = new Uint8Array(rawData.length);
    for (let i = 0; i < rawData.length; ++i) {
      outputArray[i] = rawData.charCodeAt(i);
    }
    return outputArray;
  }

  isPushSupported(): boolean {
    return this.isSupported;
  }

  async checkSubscription(): Promise<boolean> {
    if (!this.isSupported || !this.registration) {
      return false;
    }

    try {
      this.subscription = await this.registration.pushManager.getSubscription();
      return this.subscription !== null;
    } catch (error) {
      console.error('[WebPush] Failed to check subscription', error);
      return false;
    }
  }
}

export const webPushService = new WebPushService();

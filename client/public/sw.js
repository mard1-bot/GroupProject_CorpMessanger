// Service Worker for Web Push Notifications

self.addEventListener('install', (event) => {
  console.log('[SW] Install event');
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  console.log('[SW] Activate event');
  event.waitUntil(self.clients.claim());
});

self.addEventListener('push', (event) => {
  console.log('[SW] Push event received');
  
  let data = {};
  try {
    data = event.data ? event.data.json() : {};
  } catch (e) {
    console.error('Failed to parse push data:', e);
  }

  const chatId = data.ChatID || (data.Data && data.Data.chat_id) || (data.data && data.data.chat_id) || null;
  const title = data.Title || data.title || 'CorpMessenger';
  const body = data.Body || data.body || 'Новое сообщение';

  const options = {
    body: body,
    icon: '/icon.png',
    badge: '/badge.png',
    vibrate: [100, 50, 100],
    data: {
      dateOfArrival: Date.now(),
      primaryKey: 1,
      chatId: chatId,
    },
    actions: [
      {
        action: 'explore',
        title: 'Открыть',
        icon: '/icon.png',
      },
      {
        action: 'close',
        title: 'Закрыть',
        icon: '/close.png',
      },
    ],
  };

  event.waitUntil(
    self.registration.showNotification(title, options)
  );
});

self.addEventListener('notificationclick', (event) => {
  console.log('[SW] Notification click event', event);
  
  event.notification.close();

  if (event.action === 'explore' || !event.action) {
    const chatId = event.notification.data.chatId;
    const targetUrl = chatId ? `/chat/${chatId}` : '/';

    event.waitUntil(
      clients.matchAll({ type: 'window', includeUncontrolled: true }).then((clientList) => {
        // Find existing open window
        for (const client of clientList) {
          if ('focus' in client) {
            client.navigate(targetUrl);
            return client.focus();
          }
        }
        // If no window is open, open a new one
        if (clients.openWindow) {
          return clients.openWindow(targetUrl);
        }
      })
    );
  }
});

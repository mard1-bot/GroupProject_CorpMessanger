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
  
  const data = event.data ? event.data.json() : {};
  const options = {
    body: data.body || 'Новое сообщение',
    icon: '/icon.png',
    badge: '/badge.png',
    vibrate: [100, 50, 100],
    data: {
      dateOfArrival: Date.now(),
      primaryKey: 1,
      chatId: data.chatId,
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
    self.registration.showNotification(data.title || 'CorpMessenger', options)
  );
});

self.addEventListener('notificationclick', (event) => {
  console.log('[SW] Notification click event');
  
  event.notification.close();

  if (event.action === 'explore') {
    event.waitUntil(
      clients.matchAll({ type: 'window' }).then((clientList) => {
        for (const client of clientList) {
          if (client.url === '/' && 'focus' in client) {
            return client.focus();
          }
        }
        if (clients.openWindow) {
          return clients.openWindow('/');
        }
      })
    );
  }
});

# WebRTC и XMPP Setup Guide

## WebRTC (Голосовая и видеосвязь)

### Компоненты

1. **Backend (Go)**
   - `backend/internal/websocket/webrtc.go` - WebRTC signaling server
   - `backend/internal/websocket/hub.go` - добавлен CallManager
   - Поддерживает события: `call_offer`, `call_answer`, `call_ice`, `call_end`, `call_reject`, `call_accept`, `call_busy`, `call_ringing`

2. **Frontend (React Native)**
   - `client/services/calls.ts` - WebRTC сервис
   - `client/components/call-modal.tsx` - UI для звонка
   - `client/components/incoming-call.tsx` - UI для входящего звонка
   - `client/contexts/call-context.tsx` - глобальное управление звонками

### Использование

```typescript
// Начать аудиозвонок
import { callService } from '@/services/calls';
await callService.startCall(chatId, calleeId, 'audio');

// Начать видеозвонок
await callService.startCall(chatId, calleeId, 'video');

// Проверить статус звонка
const callState = callService.getCurrentCall();
```

### Функции звонка
- ✅ Аудиозвонки
- ✅ Видеозвонки  
- ✅ Входящие звонки (глобально)
- ✅ Отклонение/принятие звонка
- ✅ Mute/Unmute
- ✅ Включение/выключение видео
- ✅ ICE candidates (STUN серверы Google)

---

## XMPP (ejabberd)

### Конфигурация Docker

```yaml
# docker-compose.yml
ejabberd:
  image: ejabberd/ecs:latest
  container_name: corp-ejabberd
  environment:
    - EJABBERD_DOMAIN=localhost
    - EJABBERD_ADMINS=admin@localhost
    - EJABBERD_USERS=admin@localhost:admin
  ports:
    - "5222:5222"    # XMPP client
    - "5269:5269"    # XMPP server-to-server
    - "5280:5280"    # HTTP admin API
    - "5443:5443"    # HTTP websockets
```

### Backend интеграция

1. **XMPP Client** (`backend/internal/ejabberd/xmpp.go`)
   - Создание/удаление пользователей
   - Создание/удаление чат-комнат (MUC)
   - Управление участниками
   - Отправка сообщений

2. **Конфигурация** (.env)
   ```
   EJABBERD_HOST=localhost
   EJABBERD_PORT=5280
   EJABBERD_API_SECRET=your-ejabberd-api-secret
   ```

### API методы

```go
// Создать пользователя
xmppClient.CreateUser(userID, password)

// Создать чат-комнату
xmppClient.CreateChatRoom(roomID, title, ownerID)

// Добавить участника
xmppClient.AddMemberToRoom(roomID, userID, "member")

// Отправить сообщение
xmppClient.SendMessage(from, to, body)
```

---

## Запуск

### 1. Без XMPP (только WebRTC)

```bash
# Backend
cd backend
go run ./cmd/api

# Frontend
cd client
npm start
```

### 2. С XMPP

```bash
# Запуск всех сервисов
docker-compose up -d

# Backend автоматически подключится к ejabberd
# если заданы EJABBERD_HOST и EJABBERD_API_SECRET
```

---

## Тестирование WebRTC

1. Откройте чат с другим пользователем
2. Нажмите кнопку 📞 (аудио) или 🎥 (видео) в шапке
3. Дождитесь соединения
4. Для теста входящих звонков - позвоните с другого аккаунта

## Тестирование XMPP

```bash
# Проверка статуса ejabberd
curl http://localhost:5280/api/status

# Создание пользователя через API
curl -X POST http://localhost:5280/api/register \
  -H "Authorization: your-secret" \
  -d '{"user":"test","host":"localhost","password":"123"}'
```

---

## Архитектура

```
┌─────────────┐      WebSocket      ┌─────────────┐
│   Client A  │ ◄─────────────────► │   Backend   │
│  (WebRTC)   │                     │  (Signaling)│
└──────┬──────┘                     └──────┬──────┘
       │                                     │
       │         ┌─────────────┐             │
       └────────►│   STUN/TURN │◄────────────┘
                 │   Servers   │
                 └─────────────┘
       
┌─────────────┐      XMPP         ┌─────────────┐
│   Client B  │ ◄────────────────► │  ejabberd   │
│  (XMPP MUC) │                     │  (XMPP srv) │
└─────────────┘                     └─────────────┘
```

---

## Ограничения

- **WebRTC**: Требует HTTPS в production (getUserMedia)
- **XMPP**: ejabberd должен быть настроен с валидным SSL в production
- **STUN/TURN**: Для production рекомендуется свой TURN сервер

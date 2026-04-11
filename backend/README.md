# Go backend scaffold

Базовый каркас backend-сервиса на Go.

## Структура
- `cmd/api`
- `internal/app`
- `internal/config`
- `internal/http`
- `internal/storage`
- `internal/ejabberd`
- `internal/logger`
- `migrations`

## Запуск
```bash
cp .env.example .env
make run
```

## Эндпоинты
- `GET /health` -> 200
- `GET /ready` -> 200, пока зависимости-заглушки доступны

## Ошибки
```json
{
  "error": {
    "code": "internal",
    "message": "internal server error"
  }
}
```

## Команды
```bash
make run
make test
```

## 152-ФЗ
На следующем этапе стоит учесть согласие на обработку ПДн, хранение на территории РФ, аудит доступа, разграничение прав и шифрование каналов передачи данных.

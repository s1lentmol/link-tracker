# LinkTracker

**LinkTracker** — Telegram-бот и сервис отслеживания ссылок.

Проект состоит из двух сервисов:
- `bot` — взаимодействие с пользователем в Telegram.
- `scrapper` — хранение подписок и фоновая проверка изменений ссылок.

Взаимодействие между сервисами реализовано через **gRPC**.

## Поддерживаемые команды бота

- `/start` — регистрация пользователя.
- `/help` — список команд.
- `/track` — диалог добавления ссылки (ссылка -> теги -> фильтры).
- `/untrack` — удаление ссылки из отслеживания.
- `/list [tag]` — список отслеживаемых ссылок (опционально с фильтром по тегу).
- `/cancel` — отмена активного диалога.

## Конфигурация

В репозитории есть пример конфигурации: `.env.example`.

Создайте рабочий `.env` на его основе:

```bash
cp .env.example .env
```

И при необходимости отредактируйте значения:

```env
APP_TELEGRAM_TOKEN=your_telegram_token

# gRPC
SCRAPPER_GRPC_ADDR=:8081
BOT_GRPC_ADDR=:8082

# Интервалы/таймауты
SCHEDULER_INTERVAL=30s
HTTP_TIMEOUT=10s
GRPC_TIMEOUT=3s

# Внешние API
GITHUB_BASE_URL=https://api.github.com
STACK_BASE_URL=https://api.stackexchange.com/2.3
```

`APP_TELEGRAM_TOKEN` обязателен. Остальные параметры имеют значения по умолчанию.

## Запуск

1. Запустите scrapper:

```bash
go run ./cmd/scrapper
```

2. В отдельном терминале запустите bot:

```bash
go run ./cmd/bot
```

## Сборка и тесты

```bash
go build ./...
go test ./...
```

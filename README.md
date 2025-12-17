# Stream Engine

Бэкенд для видео-стриминга (аналог YouTube/Netflix) на Go.
Реализован как набор независимых сервисов в монолите (Modular Monolith) с возможностью масштабирования.

## Стек технологий

- **Go** 1.23+
- **PostgreSQL** (pgx/v5) - основное хранилище (пользователи, метаданные видео).
- **Kafka** - шина событий для асинхронной коммуникации.
- **Gin** - HTTP фреймворк.
- **Docker & Docker Compose** - контейнеризация и оркестрация для локальной разработки.

## Структура проекта

Архитектура: **Layered Architecture** (Слоистая).

- `cmd/api` - Точка входа HTTP API (REST).
- `cmd/worker` - Точка входа фонового воркера (обработка видео).
- `internal/app` - Слой бизнес-логики (Service) и транспортов (Handler/Consumer).
- `internal/model` - Доменные сущности.
- `internal/repository` - Слой работы с БД.
- `internal/bootstrap` - Инициализация инфраструктуры (БД, конфиги).

## Запуск

### Локально (Рекомендуется Docker Compose).

Поднять все зависимости (Postgres, Kafka, Zookeeper, Migrations):

```bash
make up
```

### Запуск сервисов

Сервер API (порт 8080):
```bash
make run
```

Фоновый воркер (обработка видео):
```bash
make worker
```

## API Эндпоинты

### Auth
- `POST /api/v1/auth/register` - Регистрация.
- `POST /api/v1/auth/login` - Вход (JWT).

### Media
- `POST /api/v1/videos/init` - Инициализация загрузки.
- `POST /api/v1/videos/finish` - Завершение загрузки (триггерит процессинг).

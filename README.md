# Stream Engine

[![CI](https://github.com/Dmitrygosu/stream-engine/actions/workflows/ci.yaml/badge.svg)](https://github.com/Dmitrygosu/stream-engine/actions/workflows/ci.yaml)
![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go)

Бэкенд загрузки и обработки видео: HTTP API принимает загрузки, фоновый воркер обрабатывает
их по событиям из Kafka. Go, PostgreSQL, Kafka, Gin.

Хранилище и транскодирование здесь имитируются: API выдаёт URL для загрузки в духе
presigned URL S3, а `SimulatedTranscoder` вместо FFmpeg ждёт пару секунд и возвращает адрес
HLS-плейлиста. Проект показывает архитектуру событийного конвейера, а не работу с медиафайлами.

## Как это работает

```
клиент ── POST /videos/init ──▶ API ── INSERT video (PENDING) ──▶ PostgreSQL
клиент ── загрузка файла по upload_url (имитация S3)
клиент ── POST /videos/finish ─▶ API ── video.uploaded ──▶ Kafka ──▶ worker
                                                                      │
                     PROCESSING → PUBLISHED (или FAILED) ◀────────────┤
                                            video.ready ◀─────────────┘
```

- **API** (`cmd/api`) — регистрация и вход (JWT), создание видео, запуск обработки,
  просмотр видео. При старте применяет миграции.
- **Worker** (`cmd/worker`) — читает `video.uploaded`, переводит видео в `PROCESSING`,
  «транскодирует», сохраняет `PUBLISHED` с адресом HLS и публикует `video.ready`.

## Гарантии

- **Доступ.** Все маршруты `/api/v1/videos` требуют JWT (HS256 с фиксированным алгоритмом,
  срок действия, `aud`, роль). Загружать могут только `CREATOR` и `ADMIN`, владелец берётся
  из токена. Завершить загрузку может только владелец или админ и только для видео в статусе
  `PENDING`. Черновики видны только владельцу и админам.
- **Как минимум однократная обработка.** Событие коммитится в Kafka только после обработки.
  Временные ошибки (база, брокер) повторяются с экспоненциальной задержкой. Событие,
  прерванное остановкой воркера, не коммитится и будет доставлено заново.
- **Идемпотентность.** Повторно доставленное событие для опубликованного видео только заново
  отправляет `video.ready` (это покрывает и падение между сохранением статуса и отправкой),
  неудавшееся видео повторно не обрабатывается. Видео не может застрять в `PROCESSING`:
  при ошибке транскодирования оно получает статус `FAILED`.
- **Надёжная запись в Kafka.** Продюсер ждёт подтверждения от всех реплик (`RequireAll`).
  Топики создаются явно сервисом `kafka-init` в docker compose.
- **Аккаунты.** Email нормализуется (регистр и пробелы). Дубликаты отсекает уникальный индекс,
  поэтому одновременные регистрации безопасны. Время ответа на вход не зависит от того,
  существует ли аккаунт. Стать `ADMIN` через регистрацию нельзя.

## Запуск

Нужен Docker с Compose v2.

```bash
make up
```

Поднимутся PostgreSQL, Kafka с топиками, API на `http://localhost:8080` и воркер.
Проверки: `/healthz`, `/readyz`.

```bash
curl -s -X POST localhost:8080/api/v1/auth/register -H 'Content-Type: application/json' -d '{"email":"ann@example.com","password":"correct horse","role":"CREATOR"}'
```

```bash
TOKEN=$(curl -s -X POST localhost:8080/api/v1/auth/login -H 'Content-Type: application/json' -d '{"email":"ann@example.com","password":"correct horse"}' | jq -r .token)
```

```bash
curl -s -X POST localhost:8080/api/v1/videos/init -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"title":"Intro"}'
```

Затем `POST /api/v1/videos/finish` с `{"video_id": "..."}` — и через пару секунд
`GET /api/v1/videos/{id}` вернёт статус `PUBLISHED` и `hls_url`.

## API

| Метод | Путь | Доступ | Описание |
|-------|------|--------|----------|
| `POST` | `/api/v1/auth/register` | все | регистрация (`VIEWER` или `CREATOR`) |
| `POST` | `/api/v1/auth/login` | все | выдаёт JWT |
| `POST` | `/api/v1/videos/init` | `CREATOR`, `ADMIN` | создаёт видео и возвращает URL для загрузки |
| `POST` | `/api/v1/videos/finish` | владелец, `ADMIN` | ставит видео в очередь на обработку (`202`) |
| `GET` | `/api/v1/videos/{id}` | авторизованные | опубликованные — всем, черновики — владельцу и админам |
| `GET` | `/healthz`, `/readyz` | все | живость и готовность |

Ошибки: `{"error": {"code": "...", "message": "..."}}` — например `invalid_request`,
`unauthorized`, `forbidden`, `video_not_found`, `upload_already_finished`, `email_taken`,
`invalid_credentials`. Внутренние ошибки логируются и не возвращаются клиенту.

## Конфигурация

| Переменная | Кому | По умолчанию | Описание |
|------------|------|--------------|----------|
| `DB_URL` | API, worker | — | строка подключения к PostgreSQL |
| `KAFKA_BROKERS` | API, worker | — | брокеры через запятую |
| `JWT_SECRET` | API | — | ключ HS256, не короче 32 байт |
| `JWT_AUDIENCE` | API | `stream-engine` | `aud` в токенах |
| `JWT_TTL` | API | `1h` | время жизни токена |
| `HTTP_PORT` | API | `8080` | порт HTTP |
| `LOGGER_LEVEL` | API, worker | `info` | уровень логирования |

Конфигурация проверяется при старте: сервис не запустится без обязательных переменных.

## Тесты

```bash
make test
```

Юнит-тесты покрывают проверку токенов (включая `alg: none` и подмену алгоритма), middleware,
правила доступа к видео, регистрацию и вход, а также цикл воркера: повторы, отказ после
исчерпания попыток без блокировки партиции, статус `FAILED`, идемпотентную повторную доставку
и отсутствие коммита при остановке. Интеграционные тесты на PostgreSQL (нужен
`TEST_DATABASE_URL`) проверяют миграции, репозитории и одновременную регистрацию с одним email.
В CI тесты идут с `-race` вместе с golangci-lint, govulncheck и сборкой образа.

## Что дальше

- настоящие presigned URL (S3/MinIO) и FFmpeg в отдельном контейнере;
- transactional outbox для `video.uploaded`, чтобы запись в базу и событие были атомарны;
- dead-letter topic для событий, исчерпавших повторы (сейчас они логируются);
- метрики Prometheus и трейсинг OpenTelemetry сквозь Kafka.

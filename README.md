# Генератор QR-кодов на Docker Compose

Микросервисное веб-приложение для генерации QR-кодов.

## Архитектура

Сервисы проекта:

- `nginx` — единая внешняя точка входа. Проксирует frontend и API.
- `frontend` — интерфейс на Vue 3, запускается через Bun + Vite.
- `gateway` — API на Go + Gin. Принимает запросы, создаёт задачи и кладёт их в очередь Asynq.
- `qrgen` — фоновый обработчик задач на Go. Забирает задания из очереди и генерирует PNG с QR-кодом.
- `redis` — брокер очередей и хранилище состояния задач/результатов.

## Как это работает

1. Пользователь открывает веб-интерфейс.
2. Frontend отправляет `POST /api/tasks` с текстом или ссылкой.
3. `gateway` создаёт запись задачи в Redis и ставит её в очередь Asynq.
4. `qrgen` получает задачу, генерирует QR-код и сохраняет PNG в Redis.
5. Frontend опрашивает `GET /api/tasks/:id` и показывает изображение после завершения.

## Запуск

```bash
docker compose up
```

После запуска приложение доступно по адресу:

- `http://localhost:8080`

При необходимости можно переопределить внешний порт:

```bash
APP_PORT=8090 docker compose up
```

## API

### Создать задачу

```http
POST /api/tasks
Content-Type: application/json

{
  "content": "https://example.com",
  "size": 256
}
```

### Получить статус задачи

```http
GET /api/tasks/{id}
```

### Скачать PNG

```http
GET /api/tasks/{id}/image
```

## CI/CD

Для проекта настроен pipeline на GitHub Actions: [.github/workflows/ci.yml](/home/danluki/projects/qrgen/.github/workflows/ci.yml)

Pipeline автоматически запускается:

- при `push` в репозиторий;
- при `pull_request`.

### Шаги pipeline

1. Клонирование репозитория.
2. Установка Go и Bun.
3. Установка зависимостей проекта.
4. Сборка backend и frontend.
5. Запуск `go test ./...`.
6. Проверка корректности `docker compose config`.
7. Smoke-test через `docker compose up -d --build` с реальным HTTP-запросом к приложению.

### Что проверяет smoke-test

Скрипт [scripts/ci-smoke.sh](/home/danluki/projects/qrgen/scripts/ci-smoke.sh):

- поднимает все контейнеры;
- использует отдельный порт `18080`, чтобы не конфликтовать с локальным запуском;
- ждёт готовности `GET /healthz`;
- создаёт задачу генерации QR-кода;
- дожидается статуса `completed`;
- скачивает PNG и убеждается, что файл не пустой.

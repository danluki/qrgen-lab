# Генератор QR-кодов на Docker Compose

Микросервисное веб-приложение для генерации QR-кодов.

## Архитектура

Сервисы проекта:

- `nginx` — единая внешняя точка входа. Проксирует frontend и API.
- `frontend` — интерфейс на Vue 3, запускается через Bun + Vite.
- `gateway` — API на Go + Gin. Принимает запросы, создаёт задачи и кладёт их в очередь Asynq.
- `qrgen` — фоновый обработчик задач на Go. Забирает задания из очереди и генерирует PNG с QR-кодом.
- `redis` — брокер очередей и хранилище состояния задач/результатов.
- `prometheus` — сбор и хранение метрик.
- `grafana` — визуализация метрик и логов.
- `loki` — централизованное хранение логов.
- `promtail` — сбор логов контейнеров Docker и отправка их в Loki.

## Как это работает

1. Пользователь открывает веб-интерфейс.
2. Frontend отправляет `POST /api/tasks` с текстом или ссылкой.
3. `gateway` создаёт запись задачи в Redis и ставит её в очередь Asynq.
4. `qrgen` получает задачу, генерирует QR-код и сохраняет PNG в Redis.
5. Frontend опрашивает `GET /api/tasks/:id` и показывает изображение после завершения.
6. Prometheus регулярно снимает метрики с `gateway` и `qrgen`.
7. Promtail собирает логи контейнеров и отправляет их в Loki.
8. Grafana показывает метрики и логи на готовом дашборде.

## Запуск

```bash
docker compose up
```

После запуска доступны:

- приложение: `http://localhost:8080`
- Grafana: `http://localhost:3000`
- Prometheus: `http://localhost:9090`

Данные для входа в Grafana:

- логин: `admin`
- пароль: `admin`

При необходимости можно переопределить внешние порты:

```bash
APP_PORT=8090 GRAFANA_PORT=3001 PROMETHEUS_PORT=9091 docker compose up
```

## Метрики

### Gateway

`gateway` экспортирует метрики на `/metrics`:

- `qrgen_gateway_http_requests_total`
- `qrgen_gateway_http_request_duration_seconds`
- `qrgen_gateway_tasks_created_total`
- `qrgen_gateway_task_enqueue_failures_total`

### QR Worker

`qrgen` экспортирует метрики на порту `2112`:

- `qrgen_worker_tasks_processed_total`
- `qrgen_worker_task_duration_seconds`
- `qrgen_worker_tasks_in_progress`

## Логи

Логи всех контейнеров собираются через Promtail из Docker и отправляются в Loki.

В Grafana можно смотреть:

- общий поток логов по проекту;
- логи конкретного сервиса по label `service`;
- логи по контейнеру через label `container`.

## Grafana

Provisioning настроен автоматически:

- datasource `Prometheus`
- datasource `Loki`
- dashboard `QRGen Overview`

Готовый дашборд содержит:

- rate HTTP-запросов gateway;
- p95 latency gateway;
- число обработанных задач worker;
- среднее время обработки задачи;
- текущее число задач в работе;
- централизованные логи контейнеров.

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

## Корректный запуск с мониторингом

```shell
APP_PORT=18080 GRAFANA_PORT=13000 PROMETHEUS_PORT=19090 docker compose up
```

## Скриншоты Для Отчёта

Плейсхолдеры для скриншотов:

- lab106 успешный CI/CD: [data/screenshots/lab106/](/home/danluki/projects/qrgen/data/screenshots/lab106)
- lab107 Grafana dashboard: [data/screenshots/lab107/](/home/danluki/projects/qrgen/data/screenshots/lab107)
- lab107 Grafana Explore / Loki logs: [data/screenshots/lab107/](/home/danluki/projects/qrgen/data/screenshots/lab107)
- lab107 Prometheus targets: [data/screenshots/lab107/](/home/danluki/projects/qrgen/data/screenshots/lab107)

Можно сохранить, например:

- `data/screenshots/lab106/ci-success.png`
- `data/screenshots/lab106/ci-failed.png`
- `data/screenshots/lab107/grafana-dashboard.png`
- `data/screenshots/lab107/grafana-logs.png`
- `data/screenshots/lab107/prometheus-targets.png`

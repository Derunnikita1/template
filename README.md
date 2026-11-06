# Trip Service

Сервис поездок с HTTP API, PostgreSQL и телеметрией OpenTelemetry.

## Требования

- Go 1.27 или новее
- Docker
- `make`
- `tripgoctl`
- `golangci-lint`
- C-компилятор для запуска тестов с `-race`

## Запуск

```bash
tripgoctl cluster start
tripgoctl environment start
tripgoctl connect
make migrate
make run
```

`tripgoctl environment start` создаёт `.env` с адресом PostgreSQL и OTLP-коллектора. Ссылки на Jaeger, Grafana и Prometheus выводит `tripgoctl connect`.

## Конфигурация

Обязательные переменные:

- `HTTP_ADDR`
- `LOG_LEVEL`
- `SHUTDOWN_TIMEOUT`
- `DATABASE_URL`
- `DATABASE_MAX_CONNS`
- `DATABASE_MIN_CONNS`
- `DATABASE_MAX_CONN_LIFETIME`
- `DATABASE_CONNECT_TIMEOUT`
- `DATABASE_QUERY_TIMEOUT`
- `OTEL_SERVICE_NAME`
- `OTEL_EXPORTER_OTLP_ENDPOINT`
- `OTEL_EXPORTER_OTLP_PROTOCOL`
- `OTEL_RESOURCE_ATTRIBUTES`
- `OTEL_TRACES_SAMPLER`

HTTP-таймауты `HTTP_READ_TIMEOUT`, `HTTP_READ_HEADER_TIMEOUT`, `HTTP_WRITE_TIMEOUT` и `HTTP_IDLE_TIMEOUT` можно переопределить. Полный пример находится в `.env.example`.

## Команды

```bash
make generate
make migrate
make migrate-down
make run
make test
make test-integration
make lint
make build
```

`make test` запускает тесты дважды с детектором гонок и печатает покрытие. `make test-integration` использует PostgreSQL из `DATABASE_URL`; перед запуском необходимо применить миграции.

## HTTP API

```bash
curl -i -X POST localhost:8080/api/v1/trips \
  -H 'content-type: application/json' \
  -d '{"user_id":"5cb72c04-7650-45c9-a79b-bcdba0631e0c","driver_id":"8860b315-ec86-42eb-a17c-7c163d721ff5","start_point":{"latitude":59.9398,"longitude":30.3146},"end_point":{"latitude":59.929,"longitude":30.3626},"price":1450}'
```

Сервис поддерживает создание, получение и завершение поездки, а также `/health` и `/ready`. Контракт находится в `contracts/openapi/trip-service.openapi.yaml`.

## Архитектура

Зависимости направлены внутрь:

```text
HTTP ──► use case ──► domain
                 ▲
repository ──────┘
infra ──► repository, container
container ──► все конкретные реализации
```

- `internal/domain/trip` содержит модель поездки, статусы, переход состояний, валидацию и доменные ошибки. Пакет не зависит от HTTP, PostgreSQL или OpenTelemetry.
- `internal/usecase/trip/<action>` содержит отдельный сценарий для create, get и finish. Интерфейсы репозитория, транзакций, часов, генератора идентификаторов и метрик объявлены в `contract.go` на стороне потребителя.
- `internal/http/trip/<action>` содержит по одному HTTP-обработчику на действие. Транспорт знает только интерфейс соответствующего use case.
- `internal/http/domainerror` является единственным местом перевода доменных ошибок в RFC 9457 Problem.
- `internal/repository/trip` реализует хранение поездок и истории статусов в PostgreSQL.
- `internal/infra/postgres` создаёт инструментированный пул, а `internal/infra/postgres/transaction` управляет транзакциями.
- `internal/infra/telemetry` создаёт логгер, OpenTelemetry SDK, экспортёры, метрики и корреляцию логов с трейсами.
- `internal/container` является composition root: создаёт конкретные реализации и связывает их. `cmd/trip-service/main.go` только загружает конфигурацию, создаёт контейнер и управляет временем жизни процесса.

Интерфейсы размещены у потребителя, потому что именно потребитель определяет минимальный набор нужных ему методов. Это сохраняет use case независимыми от конкретной базы и упрощает генерацию узких моков. При добавлении второго транспорта use case и домен не изменятся: новый транспорт будет преобразовывать свой контракт во входные данные существующих сценариев.

## Ошибки

Домен возвращает `ErrTripNotFound`, `ErrTripCompleted`, `ErrDriverBusy` и типизированные ошибки валидации. HTTP-транспорт преобразует их в стабильные коды `trip_not_found`, `trip_completed`, `driver_busy` и `invalid_request`. Неизвестные ошибки возвращаются как `internal_error` без внутренних деталей.

## Тесты

- табличные тесты доменной модели;
- unit-тесты use case с моками `gomock`;
- unit-тесты маппинга ошибок;
- тесты HTTP-обработчиков через `httptest` и моки use case;
- интеграционные тесты репозитория с реальным PostgreSQL;
- интеграционный тест полного отката вставки поездки и истории статусов.

Моки генерируются директивами `go:generate` и коммитятся рядом с тестами.

## Observability

Middleware выполняются в порядке `Recover → Tracing → Logging → Metrics → router`.

Сервис экспортирует:

- HTTP- и PostgreSQL-трейсы по OTLP;
- `http.server.request.duration`;
- `http.server.active_requests`;
- `db.client.operation.duration`;
- `tripgo.trips.created`;
- `tripgo.trips.completed`;
- `tripgo.trips.active`;
- JSON-логи `slog` с `service`, `trace_id` и `span_id`.

Для проверки нужно создать поездку, найти HTTP-трейс с дочерним PostgreSQL-спаном в Jaeger, найти метрики в Grafana и сопоставить `trace_id` из лога с трейсом. Недоступность коллектора не влияет на HTTP API и работу с PostgreSQL.

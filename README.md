# Trip Service

Лаба 1: HTTP API + PostgreSQL.

## Запуск

Нужны Go, Docker, make, tripgoctl.

```bash
tripgoctl cluster start
tripgoctl environment start
tripgoctl connect
make migrate
make run
```

Адрес берётся из `HTTP_ADDR` (обычно `:8080`).

## Env

Смотри `.env.example`. Обязательные:

- `HTTP_ADDR`
- `LOG_LEVEL`
- `SHUTDOWN_TIMEOUT`
- `DATABASE_URL`
- `DATABASE_MAX_CONNS`
- `DATABASE_MIN_CONNS`
- `DATABASE_MAX_CONN_LIFETIME`
- `DATABASE_CONNECT_TIMEOUT`
- `DATABASE_QUERY_TIMEOUT`

`.env` в git не кладём.

## Команды

```bash
make generate
make migrate
make migrate-down
make run
make test
make build
```

Пример:

```bash
curl -sS -X POST localhost:8080/api/v1/trips \
  -H 'content-type: application/json' \
  -d '{"user_id":"5cb72c04-7650-45c9-a79b-bcdba0631e0c","driver_id":"8860b315-ec86-42eb-a17c-7c163d721ff5","start_point":{"latitude":59.9398,"longitude":30.3146},"end_point":{"latitude":59.9290,"longitude":30.3626},"price":1450}'
```

## Что сделано

create / get / finish trip, `/health`, `/ready`, миграции, codegen из OpenAPI, pgx + squirrel, TxManager, `problem+json`.

## Решения

**Изоляция:** Read Committed. Для `driver_busy` хватает partial unique index, для finish — `UPDATE ... WHERE status = 'active'`.

**TxManager:** `Do` открывает tx, кладёт в context, на ошибке/панике rollback. Репозиторий сам достаёт tx или pool. Вложенный `Do` ту же tx не открывает заново.

**Две активные поездки:**

```sql
CREATE UNIQUE INDEX trips_one_active_per_driver_idx ON trips (driver_id) WHERE (status = 'active');
```

`23505` → `409 driver_busy`.

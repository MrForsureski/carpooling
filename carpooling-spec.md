# Приложение совместных поездок в офис — полная техническая документация

> Версия 1.0 | Go + HTMX + PostgreSQL/PostGIS

---

## Содержание

1. [Обзор проекта](#1-обзор-проекта)
2. [Стек технологий](#2-стек-технологий)
3. [Функциональные требования](#3-функциональные-требования)
4. [Нефункциональные требования](#4-нефункциональные-требования)
5. [Роли и права доступа](#5-роли-и-права-доступа)
6. [Ограничения поездок](#6-ограничения-поездок)
7. [Архитектура системы](#7-архитектура-системы)
8. [Структура проекта](#8-структура-проекта)
9. [База данных](#9-база-данных)
10. [API эндпоинты](#10-api-эндпоинты)
11. [Go: ключевые структуры и код](#11-go-ключевые-структуры-и-код)
12. [Фронтенд](#12-фронтенд)
13. [Интеграция с картами (OSRM + Leaflet)](#13-интеграция-с-картами-osrm--leaflet)
14. [WebSocket уведомления](#14-websocket-уведомления)
15. [Поэтапный план разработки](#15-поэтапный-план-разработки)
16. [Docker и деплой](#16-docker-и-деплой)
17. [Переменные окружения](#17-переменные-окружения)
18. [Чеклист разработчика](#18-чеклист-разработчика)

---

## 1. Обзор проекта

Веб-приложение для организации совместных поездок сотрудников одной компании в офис.

**Проблема:** сотрудники едут в офис по одному маршруту, не зная об этом, тратят деньги на бензин и создают трафик.

**Решение:** платформа, где водители публикуют свои маршруты, а пассажиры к ним присоединяются — с проверкой совместимости маршрутов, ограничениями по дальности крюка и автоматическим расчётом времени.

### Ключевые сценарии

- Иван едет в офис из района X. Создаёт поездку, указывает точку старта и время.
- Маша живёт в 1.5 км от маршрута Ивана. Видит поездку, присоединяется.
- Система проверяет: крюк для Ивана = 8 минут (допустимо), мест хватает, оба едут в один офис.
- Оба получают уведомления. Маша видит финальный маршрут с точкой подбора.

---

## 2. Стек технологий

### Бэкенд

| Компонент | Технология | Обоснование |
|---|---|---|
| Язык | **Go 1.22+** | Простой, быстрый, отличная поддержка конкурентности для WebSocket |
| HTTP роутер | **Chi v5** | Лёгкий, middleware-friendly, идиоматичный Go |
| БД драйвер | **sqlx** | Тонкая обёртка над `database/sql`, удобный scan в структуры |
| Миграции | **goose** | Простые SQL-файлы, CLI инструмент |
| Валидация | **go-playground/validator** | Декларативные теги на структурах |
| Логирование | **slog** (stdlib Go 1.21+) | Структурированные логи, нет зависимостей |
| JWT | **golang-jwt/jwt v5** | Стандарт де-факто |
| WebSocket | **gorilla/websocket** | Надёжная, зрелая библиотека |
| Конфигурация | **godotenv** + os.Getenv | Простейший подход, 12-factor app |
| HTTP клиент | stdlib `net/http` | Для запросов к OSRM API |

### Фронтенд

| Компонент | Технология | Обоснование |
|---|---|---|
| HTML шаблоны | **Go html/template** | Встроен в Go, безопасен по умолчанию |
| Интерактивность | **HTMX 1.9** | HTML-атрибуты вместо JS, минимум кода |
| Реактивный UI | **Alpine.js 3** | 15kb, локальное состояние без фреймворка |
| Карта | **Leaflet.js 1.9** | Лёгкая, хорошая документация |
| Тайлы карты | **OpenStreetMap** | Бесплатно, без ключей для разработки |
| CSS | **Tailwind CSS** (CDN) | Быстрая разработка, не нужна сборка |
| Иконки | **Heroicons** (inline SVG) | Бесплатно, inline не требует CDN |

### Инфраструктура

| Компонент | Технология |
|---|---|
| База данных | **PostgreSQL 16** + расширение **PostGIS 3** |
| Роутинг/карты | **OSRM** (публичный API `router.project-osrm.org`) |
| Контейнеризация | **Docker + Docker Compose** |
| Reverse proxy | **Caddy** (автоматический HTTPS) |
| Хостинг (старт) | **Railway** или **Render** (бесплатный tier) |

### Почему не GraphQL / gRPC / React / GORM

- **Не GraphQL:** избыточно для CRUD-приложения такого масштаба
- **Не React/Vue:** HTMX даёт 90% функциональности с 10% сложности
- **Не GORM:** скрывает SQL, сложные запросы с PostGIS пишутся вручную всё равно
- **Не Redis:** для начала хватает PostgreSQL; кэш добавим по необходимости

---

## 3. Функциональные требования

### 3.1 Регистрация и авторизация

- **FR-01:** Пользователь регистрируется через корпоративный email
- **FR-02:** Система проверяет домен email против списка разрешённых доменов компании
- **FR-03:** После регистрации — верификация email (ссылка в письме)
- **FR-04:** Логин по email + пароль, выдача JWT access token (15 мин) + refresh token (7 дней)
- **FR-05:** Refresh token ротируется при каждом обновлении (хранится в httpOnly cookie)
- **FR-06:** Выход из системы инвалидирует refresh token

### 3.2 Управление офисами (Admin)

- **FR-07:** Администратор создаёт офисы компании (название, адрес, координаты)
- **FR-08:** Для каждого офиса администратор задаёт **зону подбора** — полигон на карте, из которого водители могут начинать поездки
- **FR-09:** Администратор настраивает ограничения на уровне офиса: макс. крюк (мин), макс. расстояние до маршрута (км), мин. время до старта (мин)
- **FR-10:** Администратор может деактивировать офис (поездки в него становятся недоступны)
- **FR-11:** Офисы отображаются на общей карте для всех авторизованных пользователей

### 3.3 Поездки — водитель (Driver)

- **FR-12:** Водитель выбирает офис назначения из списка
- **FR-13:** Водитель указывает точку старта на карте (drag-and-drop маркер)
- **FR-14:** Система проверяет, что точка старта попадает в зону подбора выбранного офиса
- **FR-15:** Водитель указывает дату и время выезда
- **FR-16:** Водитель указывает количество мест (1–6, не считая себя)
- **FR-17:** Система запрашивает OSRM, получает маршрут (GeoJSON polyline) и примерное время
- **FR-18:** Водитель видит маршрут на карте до сохранения (preview)
- **FR-19:** Водитель может отменить поездку до выезда; все пассажиры получают уведомления
- **FR-20:** Водитель видит список своих пассажиров с именами и точками подбора
- **FR-21:** Водитель не может создать две поездки с пересечением времени (±2 часа)

### 3.4 Поездки — пассажир (Passenger)

- **FR-22:** Пассажир выбирает офис и дату для поиска поездок
- **FR-23:** Список поездок показывает: водитель, время, маршрут, свободные места, примерное время в пути
- **FR-24:** Пассажир указывает свою точку подбора на карте
- **FR-25:** Система проверяет все ограничения перед присоединением (см. раздел 6)
- **FR-26:** При успешной проверке место бронируется; водитель получает уведомление
- **FR-27:** Пассажир видит финальный маршрут (включая точку подбора) на карте
- **FR-28:** Пассажир может покинуть поездку не позднее чем за 30 минут до старта
- **FR-29:** Пассажир видит историю своих поездок

### 3.5 Карта

- **FR-30:** Интерактивная карта Leaflet с маркерами офисов
- **FR-31:** При клике на офис — popup с информацией и кнопкой "Найти поездки"
- **FR-32:** Маршрут поездки отображается как синяя polyline
- **FR-33:** Точка старта водителя — зелёный маркер, офис — красный, точки пассажиров — жёлтые
- **FR-34:** Зона подбора офиса отображается как полупрозрачный полигон

---

## 4. Нефункциональные требования

### Безопасность

- **NFR-01:** Все пароли хранятся как bcrypt hash (cost=12)
- **NFR-02:** JWT подписываются RS256 (ассиметричный ключ)
- **NFR-03:** Все эндпоинты кроме `/auth/*` требуют валидный JWT
- **NFR-04:** Role-based access control через middleware на уровне роутера
- **NFR-05:** SQL-инъекции исключены — только параметризованные запросы
- **NFR-06:** CORS настроен только на разрешённые домены
- **NFR-07:** Rate limiting: 20 req/min на `/auth/login` (защита от брутфорса)
- **NFR-08:** Refresh token хранится в httpOnly + Secure cookie (защита от XSS)

### Производительность

- **NFR-09:** Время ответа API: p95 < 200ms для основных операций
- **NFR-10:** PostGIS индексы на всех геометрических полях (`GIST`)
- **NFR-11:** Пагинация на всех списках (limit/offset, default limit=20)
- **NFR-12:** WebSocket соединение — одна горутина на пользователя

### Надёжность

- **NFR-13:** Проверка ограничений при join выполняется в одной транзакции (SELECT FOR UPDATE на запись о поездке)
- **NFR-14:** Graceful shutdown: Go сервер ждёт завершения активных запросов до 30 сек
- **NFR-15:** Health check эндпоинт `/health` для оркестраторов

### Наблюдаемость

- **NFR-16:** Структурированные JSON логи через `slog` (уровни: DEBUG, INFO, WARN, ERROR)
- **NFR-17:** Каждый запрос логируется: метод, путь, статус, latency, user_id
- **NFR-18:** Ошибки внешних API (OSRM) логируются с полным контекстом

---

## 5. Роли и права доступа

```
unverified → verified (passenger) → driver → admin
```

Роль `driver` назначается администратором вручную или автоматически (настраивается).

### Матрица доступа

| Действие | unverified | passenger | driver | admin |
|---|:---:|:---:|:---:|:---:|
| Просмотр карты с офисами | ✓ | ✓ | ✓ | ✓ |
| Просмотр списка поездок | — | ✓ | ✓ | ✓ |
| Присоединиться к поездке | — | ✓ | ✓ | ✓ |
| Создать поездку | — | — | ✓ | ✓ |
| Отменить свою поездку | — | — | ✓ | ✓ |
| Управление офисами | — | — | — | ✓ |
| Управление зонами офиса | — | — | — | ✓ |
| Настройка ограничений | — | — | — | ✓ |
| Управление пользователями | — | — | — | ✓ |
| Назначение роли driver | — | — | — | ✓ |

### Middleware цепочка

```
Request
  → LoggingMiddleware       (всегда)
  → RateLimitMiddleware     (только /auth/*)
  → AuthMiddleware          (все кроме /auth/*, /health)
  → RoleMiddleware(role)    (конкретные роуты)
  → Handler
```

---

## 6. Ограничения поездок

### 6.1 Ограничения для водителя

| Параметр | Значение | Настраивается |
|---|---|---|
| Мест в машине | 1–6 | Водитель при создании |
| Минимальное время до старта для создания | 60 минут | Глобально admin |
| Не более 1 активной поездки | да | Жёстко |
| Точка старта в зоне офиса | да | Зона задаётся admin |

### 6.2 Ограничения для пассажира (проверяются при join)

**Шаг 1 — Базовые проверки (быстрые, без OSRM):**

```
1. trips.seats_left > 0                          -- есть места
2. trips.depart_at > NOW() + 30 min              -- не слишком поздно
3. trips.status = 'scheduled'                    -- поездка активна
4. пассажир не водитель этой поездки             -- нельзя ехать с собой
5. пассажир ещё не в этой поездке                -- дедупликация
6. пассажир не в другой поездке в то же время    -- конфликт расписания
```

**Шаг 2 — Геометрические проверки (PostGIS):**

```sql
-- Расстояние от точки пассажира до ближайшей точки маршрута
SELECT ST_Distance(
    pickup_point::geography,
    ST_GeomFromGeoJSON(route_geojson)::geography
) AS distance_meters
```

- Если `distance_meters > office_zones.max_distance_meters` → отказ

**Шаг 3 — Расчёт крюка (OSRM):**

```
Оригинальный маршрут: старт водителя → офис = T_original
Новый маршрут: старт водителя → точка пассажира → офис = T_new
Крюк = T_new - T_original
```

- Если `крюк > office_zones.max_detour_minutes` → отказ

**Итоговая таблица ограничений с дефолтами:**

| Параметр | Дефолт | Мин | Макс |
|---|---|---|---|
| Макс. расстояние до маршрута | 2000 м | 100 м | 10 000 м |
| Макс. крюк для водителя | 15 мин | 1 мин | 60 мин |
| Мин. время до старта для join | 30 мин | 5 мин | 180 мин |
| Мин. время до старта для отмены пасс. | 30 мин | 5 мин | 180 мин |
| Макс. пассажиров | 6 | 1 | 6 |

---

## 7. Архитектура системы

### Общая схема

```
┌─────────────────────────────────────────┐
│           Browser (HTMX + Leaflet)       │
└───────────────────┬─────────────────────┘
                    │ HTTP / WebSocket
                    ▼
┌─────────────────────────────────────────┐
│              Caddy (reverse proxy)       │
│              TLS termination             │
└───────────────────┬─────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────┐
│           Go Application Server          │
│  ┌──────────┐ ┌──────────┐ ┌─────────┐  │
│  │  Router  │ │   Auth   │ │  WS Hub │  │
│  │  (Chi)   │ │Middleware│ │         │  │
│  └────┬─────┘ └──────────┘ └─────────┘  │
│       │                                  │
│  ┌────▼──────────────────────────────┐  │
│  │           Service Layer            │  │
│  │  AuthSvc │ TripSvc │ OfficeSvc    │  │
│  └────┬──────────────────────────────┘  │
│       │                                  │
│  ┌────▼──────────────────────────────┐  │
│  │         Repository Layer           │  │
│  │  UserRepo │ TripRepo │ OfficeRepo  │  │
│  └────┬──────────────────────────────┘  │
└───────┼─────────────────────────────────┘
        │                    │ HTTP
        ▼                    ▼
┌──────────────┐    ┌────────────────┐
│ PostgreSQL   │    │  OSRM API      │
│ + PostGIS    │    │ (внешний)      │
└──────────────┘    └────────────────┘
```

### Слои приложения

**Handler** — принимает HTTP запрос, парсит параметры, вызывает Service, рендерит шаблон или возвращает JSON.

**Service** — бизнес-логика: проверка ограничений, вызов внешних API, оркестрация репозиториев.

**Repository** — SQL запросы к БД. Никакой бизнес-логики, только CRUD и специфичные выборки.

**Model** — Go структуры, соответствующие таблицам БД и JSON ответам.

---

## 8. Структура проекта

```
carpooling/
├── cmd/
│   └── server/
│       └── main.go                 # точка входа, инициализация
│
├── internal/
│   ├── config/
│   │   └── config.go               # загрузка конфига из env
│   │
│   ├── model/
│   │   ├── user.go                 # User, Role
│   │   ├── trip.go                 # Trip, TripPassenger, TripStatus
│   │   ├── office.go               # Office, OfficeZone, ZoneSettings
│   │   └── errors.go               # кастомные ошибки приложения
│   │
│   ├── repository/
│   │   ├── repository.go           # интерфейсы репозиториев
│   │   ├── user_repo.go
│   │   ├── trip_repo.go
│   │   ├── office_repo.go
│   │   └── postgres/               # конкретные реализации
│   │       ├── user_repo.go
│   │       ├── trip_repo.go
│   │       └── office_repo.go
│   │
│   ├── service/
│   │   ├── auth_service.go         # регистрация, логин, JWT
│   │   ├── trip_service.go         # создание, join, отмена
│   │   ├── office_service.go       # CRUD офисов, зоны
│   │   ├── routing_service.go      # запросы к OSRM
│   │   └── notification_service.go # WebSocket уведомления
│   │
│   ├── handler/
│   │   ├── handler.go              # общая структура Handler
│   │   ├── auth_handler.go         # /auth/*
│   │   ├── trip_handler.go         # /trips/*
│   │   ├── office_handler.go       # /offices/*
│   │   ├── admin_handler.go        # /admin/*
│   │   └── ws_handler.go           # /ws
│   │
│   ├── middleware/
│   │   ├── auth.go                 # JWT проверка
│   │   ├── role.go                 # role-based access
│   │   ├── logging.go              # request logging
│   │   └── ratelimit.go            # rate limiter
│   │
│   └── websocket/
│       └── hub.go                  # WebSocket hub, broadcast
│
├── migrations/
│   ├── 001_create_companies.sql
│   ├── 002_create_users.sql
│   ├── 003_create_offices.sql
│   ├── 004_create_office_zones.sql
│   ├── 005_create_trips.sql
│   └── 006_create_trip_passengers.sql
│
├── web/
│   ├── templates/
│   │   ├── layout/
│   │   │   ├── base.html           # базовый layout
│   │   │   └── nav.html            # навигация
│   │   ├── auth/
│   │   │   ├── login.html
│   │   │   └── register.html
│   │   ├── trips/
│   │   │   ├── list.html           # список поездок
│   │   │   ├── create.html         # форма создания
│   │   │   └── detail.html         # детали поездки
│   │   ├── map/
│   │   │   └── index.html          # главная карта
│   │   └── admin/
│   │       ├── offices.html
│   │       └── users.html
│   │
│   └── static/
│       ├── css/
│       │   └── app.css
│       └── js/
│           └── map.js              # инициализация Leaflet
│
├── .env.example
├── .env
├── docker-compose.yml
├── Dockerfile
├── go.mod
├── go.sum
└── Makefile
```

---

## 9. База данных

### Расширения PostgreSQL

```sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS postgis;
```

### Миграция 001 — companies

```sql
-- migrations/001_create_companies.sql
-- +goose Up

CREATE TABLE companies (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(255) NOT NULL,
    email_domain VARCHAR(100) NOT NULL UNIQUE,  -- например "acme.com"
    is_active   BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_companies_email_domain ON companies(email_domain);

-- +goose Down
DROP TABLE companies;
```

### Миграция 002 — users

```sql
-- migrations/002_create_users.sql
-- +goose Up

CREATE TYPE user_role AS ENUM ('unverified', 'passenger', 'driver', 'admin');

CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    company_id      UUID NOT NULL REFERENCES companies(id) ON DELETE RESTRICT,
    email           VARCHAR(255) NOT NULL UNIQUE,
    password_hash   VARCHAR(255) NOT NULL,
    full_name       VARCHAR(255) NOT NULL,
    phone           VARCHAR(20),
    role            user_role NOT NULL DEFAULT 'unverified',
    email_verified  BOOLEAN NOT NULL DEFAULT false,
    verify_token    VARCHAR(255),                    -- одноразовый токен для email
    verify_token_at TIMESTAMPTZ,                     -- когда выслали
    refresh_token   VARCHAR(255),                    -- текущий refresh token (хэш)
    refresh_token_at TIMESTAMPTZ,
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_company_id ON users(company_id);
CREATE INDEX idx_users_role ON users(role);

-- +goose Down
DROP TABLE users;
DROP TYPE user_role;
```

### Миграция 003 — offices

```sql
-- migrations/003_create_offices.sql
-- +goose Up

CREATE TABLE offices (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    company_id  UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    name        VARCHAR(255) NOT NULL,
    address     TEXT NOT NULL,
    location    GEOMETRY(Point, 4326) NOT NULL,  -- координаты офиса
    is_active   BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_offices_company_id ON offices(company_id);
CREATE INDEX idx_offices_location ON offices USING GIST(location);

-- +goose Down
DROP TABLE offices;
```

### Миграция 004 — office_zones (ограничения)

```sql
-- migrations/004_create_office_zones.sql
-- +goose Up

CREATE TABLE office_zones (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    office_id               UUID NOT NULL REFERENCES offices(id) ON DELETE CASCADE,
    -- полигон зоны, из которой водители могут начинать поездку
    pickup_zone             GEOMETRY(Polygon, 4326) NOT NULL,
    -- ограничения (можно менять без деплоя)
    max_detour_minutes      INTEGER NOT NULL DEFAULT 15
                                CHECK (max_detour_minutes BETWEEN 1 AND 60),
    max_distance_meters     INTEGER NOT NULL DEFAULT 2000
                                CHECK (max_distance_meters BETWEEN 100 AND 10000),
    min_join_minutes        INTEGER NOT NULL DEFAULT 30
                                CHECK (min_join_minutes BETWEEN 5 AND 180),
    min_cancel_minutes      INTEGER NOT NULL DEFAULT 30
                                CHECK (min_cancel_minutes BETWEEN 5 AND 180),
    max_seats               INTEGER NOT NULL DEFAULT 6
                                CHECK (max_seats BETWEEN 1 AND 6),
    is_active               BOOLEAN NOT NULL DEFAULT true,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_office_zones_office_id ON office_zones(office_id);
CREATE INDEX idx_office_zones_pickup_zone ON office_zones USING GIST(pickup_zone);

-- +goose Down
DROP TABLE office_zones;
```

### Миграция 005 — trips

```sql
-- migrations/005_create_trips.sql
-- +goose Up

CREATE TYPE trip_status AS ENUM (
    'scheduled',    -- запланирована, можно присоединиться
    'in_progress',  -- водитель выехал
    'completed',    -- завершена
    'cancelled'     -- отменена водителем
);

CREATE TABLE trips (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    driver_id       UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    office_id       UUID NOT NULL REFERENCES offices(id) ON DELETE RESTRICT,
    origin          GEOMETRY(Point, 4326) NOT NULL,   -- точка старта водителя
    origin_address  TEXT,                              -- строковый адрес (для UI)
    depart_at       TIMESTAMPTZ NOT NULL,
    seats_total     INTEGER NOT NULL CHECK (seats_total BETWEEN 1 AND 6),
    seats_left      INTEGER NOT NULL CHECK (seats_left >= 0),
    -- маршрут от OSRM: GeoJSON LineString
    route_geojson   TEXT NOT NULL,
    -- время в пути от OSRM (секунды)
    duration_seconds INTEGER NOT NULL,
    -- расстояние в метрах
    distance_meters  INTEGER NOT NULL,
    status          trip_status NOT NULL DEFAULT 'scheduled',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT seats_left_lte_total CHECK (seats_left <= seats_total)
);

CREATE INDEX idx_trips_driver_id ON trips(driver_id);
CREATE INDEX idx_trips_office_id ON trips(office_id);
CREATE INDEX idx_trips_depart_at ON trips(depart_at);
CREATE INDEX idx_trips_status ON trips(status);
CREATE INDEX idx_trips_origin ON trips USING GIST(origin);

-- +goose Down
DROP TABLE trips;
DROP TYPE trip_status;
```

### Миграция 006 — trip_passengers

```sql
-- migrations/006_create_trip_passengers.sql
-- +goose Up

CREATE TYPE passenger_status AS ENUM (
    'confirmed',    -- подтверждён
    'cancelled'     -- пассажир отказался
);

CREATE TABLE trip_passengers (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    trip_id     UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    -- точка подбора пассажира
    pickup      GEOMETRY(Point, 4326) NOT NULL,
    pickup_address TEXT,
    -- крюк, который водитель делает ради этого пассажира (секунды)
    detour_seconds INTEGER NOT NULL DEFAULT 0,
    status      passenger_status NOT NULL DEFAULT 'confirmed',
    joined_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cancelled_at TIMESTAMPTZ,

    UNIQUE(trip_id, user_id)   -- нельзя быть в одной поездке дважды
);

CREATE INDEX idx_trip_passengers_trip_id ON trip_passengers(trip_id);
CREATE INDEX idx_trip_passengers_user_id ON trip_passengers(user_id);
CREATE INDEX idx_trip_passengers_pickup ON trip_passengers USING GIST(pickup);

-- +goose Down
DROP TABLE trip_passengers;
DROP TYPE passenger_status;
```

### Ключевые SQL запросы

**Поиск поездок с пространственной фильтрацией:**

```sql
-- Поездки в офис X на дату D, где пассажир в радиусе R от маршрута
SELECT
    t.id,
    t.driver_id,
    u.full_name AS driver_name,
    t.depart_at,
    t.seats_left,
    t.duration_seconds,
    t.distance_meters,
    t.route_geojson,
    -- расстояние от точки пассажира до маршрута
    ST_Distance(
        ST_GeomFromText($3, 4326)::geography,
        ST_GeomFromGeoJSON(t.route_geojson)::geography
    ) AS distance_to_route
FROM trips t
JOIN users u ON u.id = t.driver_id
WHERE
    t.office_id = $1
    AND t.depart_at::date = $2::date
    AND t.status = 'scheduled'
    AND t.seats_left > 0
    AND t.depart_at > NOW() + INTERVAL '30 minutes'
    -- только поездки, маршрут которых проходит достаточно близко
    AND ST_DWithin(
        ST_GeomFromText($3, 4326)::geography,
        ST_GeomFromGeoJSON(t.route_geojson)::geography,
        $4  -- max_distance_meters из office_zones
    )
ORDER BY t.depart_at ASC, distance_to_route ASC;
```

**Проверка конфликта расписания у пассажира:**

```sql
SELECT COUNT(*) FROM trip_passengers tp
JOIN trips t ON t.id = tp.trip_id
WHERE
    tp.user_id = $1
    AND tp.status = 'confirmed'
    AND t.status = 'scheduled'
    AND t.depart_at BETWEEN ($2::timestamptz - INTERVAL '2 hours')
                        AND ($2::timestamptz + INTERVAL '2 hours');
```

**Проверка точки старта в зоне офиса:**

```sql
SELECT oz.id, oz.max_detour_minutes, oz.max_distance_meters, oz.min_join_minutes
FROM office_zones oz
WHERE
    oz.office_id = $1
    AND oz.is_active = true
    AND ST_Within(
        ST_GeomFromText($2, 4326),  -- точка водителя
        oz.pickup_zone
    )
LIMIT 1;
```

---

## 10. API эндпоинты

### Авторизация

```
POST /auth/register
POST /auth/login
POST /auth/logout
POST /auth/refresh
GET  /auth/verify?token=xxx
```

**POST /auth/register**
```json
// Request
{
  "email": "ivan@acme.com",
  "password": "SecurePass123!",
  "full_name": "Иван Петров",
  "phone": "+79001234567"
}

// Response 201
{
  "id": "uuid",
  "email": "ivan@acme.com",
  "message": "Проверьте почту для подтверждения email"
}

// Response 400 — домен не разрешён
{
  "error": "email_domain_not_allowed",
  "message": "Домен gmail.com не разрешён"
}
```

**POST /auth/login**
```json
// Request
{
  "email": "ivan@acme.com",
  "password": "SecurePass123!"
}

// Response 200
{
  "access_token": "eyJ...",
  "expires_in": 900,
  "user": {
    "id": "uuid",
    "full_name": "Иван Петров",
    "role": "driver"
  }
}
// refresh_token устанавливается в httpOnly cookie
```

### Офисы

```
GET    /offices              — список офисов компании
GET    /offices/:id          — детали офиса
POST   /admin/offices        — создать офис [admin]
PUT    /admin/offices/:id    — обновить офис [admin]
DELETE /admin/offices/:id    — деактивировать [admin]
POST   /admin/offices/:id/zones     — создать зону [admin]
PUT    /admin/offices/:id/zones/:zid — обновить ограничения [admin]
```

**GET /offices**
```json
// Response 200
{
  "offices": [
    {
      "id": "uuid",
      "name": "Офис Москва Центр",
      "address": "ул. Тверская, 1",
      "lat": 55.7614,
      "lng": 37.6186,
      "is_active": true
    }
  ]
}
```

**POST /admin/offices**
```json
// Request
{
  "name": "Офис Москва Север",
  "address": "Ленинградский пр-т, 80",
  "lat": 55.8083,
  "lng": 37.5153
}
```

**POST /admin/offices/:id/zones**
```json
// Request — полигон зоны + ограничения
{
  "pickup_zone": {
    "type": "Polygon",
    "coordinates": [[[37.5, 55.7], [37.6, 55.7], [37.6, 55.8], [37.5, 55.8], [37.5, 55.7]]]
  },
  "max_detour_minutes": 15,
  "max_distance_meters": 2000,
  "min_join_minutes": 30
}
```

### Поездки

```
GET  /trips              — поиск поездок (query params: office_id, date, lat, lng)
GET  /trips/:id          — детали поездки
POST /trips              — создать поездку [driver]
POST /trips/:id/join     — присоединиться [passenger, driver]
POST /trips/:id/leave    — покинуть поездку
POST /trips/:id/cancel   — отменить поездку [driver, admin]
GET  /trips/my           — мои поездки (как водитель)
GET  /trips/joined       — поездки, в которых я пассажир
```

**GET /trips?office_id=uuid&date=2025-03-15&lat=55.75&lng=37.62**
```json
// Response 200
{
  "trips": [
    {
      "id": "uuid",
      "driver": {
        "id": "uuid",
        "full_name": "Иван Петров"
      },
      "office": {
        "id": "uuid",
        "name": "Офис Центр"
      },
      "depart_at": "2025-03-15T08:30:00+03:00",
      "seats_left": 2,
      "seats_total": 3,
      "duration_seconds": 1800,
      "distance_meters": 12500,
      "distance_to_route": 850,
      "route_geojson": "{...}"
    }
  ],
  "total": 5,
  "page": 1
}
```

**POST /trips**
```json
// Request
{
  "office_id": "uuid",
  "origin_lat": 55.75,
  "origin_lng": 37.62,
  "origin_address": "ул. Садовая, 5",
  "depart_at": "2025-03-15T08:30:00+03:00",
  "seats_total": 3
}

// Response 201
{
  "id": "uuid",
  "route_geojson": "{...}",
  "duration_seconds": 1800,
  "distance_meters": 12500,
  "message": "Поездка создана"
}

// Response 400 — точка вне зоны
{
  "error": "origin_outside_zone",
  "message": "Точка старта находится вне допустимой зоны для этого офиса"
}
```

**POST /trips/:id/join**
```json
// Request
{
  "pickup_lat": 55.77,
  "pickup_lng": 37.63,
  "pickup_address": "ул. Ленина, 10"
}

// Response 200
{
  "message": "Вы присоединились к поездке",
  "detour_seconds": 480,
  "pickup_geojson": "{...}"
}

// Response 400 — нарушено ограничение
{
  "error": "detour_exceeded",
  "message": "Крюк для водителя составит 22 мин, максимум 15 мин"
}

// Response 400 — нет мест
{
  "error": "no_seats_available",
  "message": "В этой поездке нет свободных мест"
}

// Response 400 — слишком поздно
{
  "error": "join_too_late",
  "message": "Присоединиться можно не позднее чем за 30 мин до выезда"
}

// Response 400 — далеко от маршрута
{
  "error": "too_far_from_route",
  "message": "Ваша точка подбора в 3.2 км от маршрута, максимум 2 км"
}
```

### WebSocket

```
GET /ws   — установить WebSocket соединение (требует JWT в query param ?token=)
```

**Формат сообщений (сервер → клиент):**
```json
{
  "type": "passenger_joined",
  "data": {
    "trip_id": "uuid",
    "passenger": {
      "id": "uuid",
      "full_name": "Мария Сидорова"
    },
    "seats_left": 1
  }
}

{
  "type": "trip_cancelled",
  "data": {
    "trip_id": "uuid",
    "message": "Водитель отменил поездку"
  }
}

{
  "type": "passenger_left",
  "data": {
    "trip_id": "uuid",
    "passenger_id": "uuid",
    "seats_left": 2
  }
}
```

### Административная панель

```
GET  /admin/users               — список пользователей
PUT  /admin/users/:id/role      — изменить роль
GET  /admin/trips               — все поездки (с фильтрами)
POST /admin/trips/:id/cancel    — принудительная отмена
GET  /admin/stats               — статистика: поездки, пользователи
```

---

## 11. Go: ключевые структуры и код

### main.go

```go
package main

import (
    "context"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/yourcompany/carpooling/internal/config"
    "github.com/yourcompany/carpooling/internal/handler"
    "github.com/yourcompany/carpooling/internal/repository/postgres"
    "github.com/yourcompany/carpooling/internal/service"
    "github.com/yourcompany/carpooling/internal/websocket"
    "github.com/jmoiron/sqlx"
    _ "github.com/lib/pq"
)

func main() {
    // Логгер
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))
    slog.SetDefault(logger)

    // Конфиг
    cfg := config.Load()

    // БД
    db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
    if err != nil {
        slog.Error("failed to connect to database", "error", err)
        os.Exit(1)
    }
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(5 * time.Minute)
    defer db.Close()

    // Репозитории
    userRepo := postgres.NewUserRepo(db)
    tripRepo := postgres.NewTripRepo(db)
    officeRepo := postgres.NewOfficeRepo(db)

    // WebSocket Hub
    hub := websocket.NewHub()
    go hub.Run()

    // Сервисы
    authSvc := service.NewAuthService(userRepo, cfg)
    routingSvc := service.NewRoutingService(cfg.OSRMBaseURL)
    officeSvc := service.NewOfficeService(officeRepo)
    notifSvc := service.NewNotificationService(hub)
    tripSvc := service.NewTripService(tripRepo, officeRepo, routingSvc, notifSvc, db)

    // Хэндлеры
    h := handler.New(handler.Deps{
        AuthSvc:   authSvc,
        TripSvc:   tripSvc,
        OfficeSvc: officeSvc,
        NotifSvc:  notifSvc,
        Hub:       hub,
        Config:    cfg,
    })

    // HTTP сервер
    srv := &http.Server{
        Addr:         ":" + cfg.Port,
        Handler:      h.Router(),
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    // Graceful shutdown
    go func() {
        slog.Info("server starting", "port", cfg.Port)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            slog.Error("server error", "error", err)
            os.Exit(1)
        }
    }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    slog.Info("shutting down server...")
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    if err := srv.Shutdown(ctx); err != nil {
        slog.Error("server forced to shutdown", "error", err)
    }
    slog.Info("server exited")
}
```

### model/trip.go

```go
package model

import (
    "time"
    "github.com/google/uuid"
)

type TripStatus string
const (
    TripStatusScheduled  TripStatus = "scheduled"
    TripStatusInProgress TripStatus = "in_progress"
    TripStatusCompleted  TripStatus = "completed"
    TripStatusCancelled  TripStatus = "cancelled"
)

type Trip struct {
    ID              uuid.UUID  `db:"id"               json:"id"`
    DriverID        uuid.UUID  `db:"driver_id"        json:"driver_id"`
    OfficeID        uuid.UUID  `db:"office_id"        json:"office_id"`
    // WKT точки: "SRID=4326;POINT(37.62 55.75)"
    OriginWKT       string     `db:"origin"           json:"-"`
    OriginLat       float64    `db:"-"                json:"origin_lat"`
    OriginLng       float64    `db:"-"                json:"origin_lng"`
    OriginAddress   string     `db:"origin_address"   json:"origin_address"`
    DepartAt        time.Time  `db:"depart_at"        json:"depart_at"`
    SeatsTotal      int        `db:"seats_total"      json:"seats_total"`
    SeatsLeft       int        `db:"seats_left"       json:"seats_left"`
    RouteGeoJSON    string     `db:"route_geojson"    json:"route_geojson"`
    DurationSeconds int        `db:"duration_seconds" json:"duration_seconds"`
    DistanceMeters  int        `db:"distance_meters"  json:"distance_meters"`
    Status          TripStatus `db:"status"           json:"status"`
    CreatedAt       time.Time  `db:"created_at"       json:"created_at"`
    UpdatedAt       time.Time  `db:"updated_at"       json:"updated_at"`

    // Joined поля (не из таблицы trips)
    DriverName       string  `db:"driver_name"        json:"driver_name,omitempty"`
    OfficeName       string  `db:"office_name"        json:"office_name,omitempty"`
    DistanceToRoute  float64 `db:"distance_to_route"  json:"distance_to_route,omitempty"`
}

type TripPassenger struct {
    ID             uuid.UUID `db:"id"              json:"id"`
    TripID         uuid.UUID `db:"trip_id"         json:"trip_id"`
    UserID         uuid.UUID `db:"user_id"         json:"user_id"`
    PickupWKT      string    `db:"pickup"          json:"-"`
    PickupLat      float64   `db:"-"               json:"pickup_lat"`
    PickupLng      float64   `db:"-"               json:"pickup_lng"`
    PickupAddress  string    `db:"pickup_address"  json:"pickup_address"`
    DetourSeconds  int       `db:"detour_seconds"  json:"detour_seconds"`
    Status         string    `db:"status"          json:"status"`
    JoinedAt       time.Time `db:"joined_at"       json:"joined_at"`

    // Joined
    UserFullName string `db:"user_full_name" json:"user_full_name,omitempty"`
}

// Запрос на создание поездки
type CreateTripRequest struct {
    OfficeID      string  `json:"office_id"      validate:"required,uuid"`
    OriginLat     float64 `json:"origin_lat"     validate:"required,latitude"`
    OriginLng     float64 `json:"origin_lng"     validate:"required,longitude"`
    OriginAddress string  `json:"origin_address" validate:"required,max=500"`
    DepartAt      string  `json:"depart_at"      validate:"required"`
    SeatsTotal    int     `json:"seats_total"    validate:"required,min=1,max=6"`
}

// Запрос на присоединение
type JoinTripRequest struct {
    PickupLat     float64 `json:"pickup_lat"     validate:"required,latitude"`
    PickupLng     float64 `json:"pickup_lng"     validate:"required,longitude"`
    PickupAddress string  `json:"pickup_address" validate:"required,max=500"`
}
```

### service/trip_service.go

```go
package service

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/jmoiron/sqlx"
    "github.com/yourcompany/carpooling/internal/model"
)

type TripService struct {
    tripRepo   TripRepository
    officeRepo OfficeRepository
    routing    *RoutingService
    notif      *NotificationService
    db         *sqlx.DB
}

// CreateTrip создаёт новую поездку
func (s *TripService) CreateTrip(ctx context.Context, driverID uuid.UUID, req model.CreateTripRequest) (*model.Trip, error) {
    officeID, _ := uuid.Parse(req.OfficeID)
    departAt, err := time.Parse(time.RFC3339, req.DepartAt)
    if err != nil {
        return nil, fmt.Errorf("invalid depart_at format: %w", err)
    }

    // 1. Проверяем минимальное время до старта
    if time.Until(departAt) < 60*time.Minute {
        return nil, model.ErrTooSoonToCreate
    }

    // 2. Проверяем, что точка старта в зоне офиса
    zone, err := s.officeRepo.FindZoneContainingPoint(ctx, officeID, req.OriginLat, req.OriginLng)
    if err != nil {
        return nil, model.ErrOriginOutsideZone
    }

    // 3. Проверяем, нет ли активной поездки в это время
    conflict, err := s.tripRepo.HasSchedulingConflict(ctx, driverID, departAt, "driver")
    if err != nil {
        return nil, err
    }
    if conflict {
        return nil, model.ErrSchedulingConflict
    }

    // 4. Получаем офис для координат назначения
    office, err := s.officeRepo.GetByID(ctx, officeID)
    if err != nil {
        return nil, err
    }

    // 5. Запрашиваем маршрут у OSRM
    route, err := s.routing.GetRoute(ctx,
        req.OriginLat, req.OriginLng,
        office.Lat, office.Lng,
    )
    if err != nil {
        return nil, fmt.Errorf("routing service error: %w", err)
    }

    // 6. Создаём поездку в БД
    trip := &model.Trip{
        ID:              uuid.New(),
        DriverID:        driverID,
        OfficeID:        officeID,
        OriginLat:       req.OriginLat,
        OriginLng:       req.OriginLng,
        OriginAddress:   req.OriginAddress,
        DepartAt:        departAt,
        SeatsTotal:      req.SeatsTotal,
        SeatsLeft:       req.SeatsTotal,
        RouteGeoJSON:    route.GeoJSON,
        DurationSeconds: route.DurationSeconds,
        DistanceMeters:  route.DistanceMeters,
        Status:          model.TripStatusScheduled,
        ZoneID:          zone.ID,
    }

    if err := s.tripRepo.Create(ctx, trip); err != nil {
        return nil, err
    }

    return trip, nil
}

// JoinTrip — присоединение пассажира к поездке
func (s *TripService) JoinTrip(ctx context.Context, passengerID uuid.UUID, tripID uuid.UUID, req model.JoinTripRequest) (*model.TripPassenger, error) {
    // Используем транзакцию, чтобы не было race condition на места
    tx, err := s.db.BeginTxx(ctx, nil)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()

    // SELECT FOR UPDATE — блокируем запись поездки
    trip, err := s.tripRepo.GetForUpdateTx(ctx, tx, tripID)
    if err != nil {
        return nil, model.ErrTripNotFound
    }

    // --- Базовые проверки ---

    if trip.Status != model.TripStatusScheduled {
        return nil, model.ErrTripNotScheduled
    }
    if trip.SeatsLeft <= 0 {
        return nil, model.ErrNoSeatsAvailable
    }
    if trip.DriverID == passengerID {
        return nil, model.ErrCannotJoinOwnTrip
    }
    if time.Until(trip.DepartAt) < time.Duration(trip.Zone.MinJoinMinutes)*time.Minute {
        return nil, model.ErrJoinTooLate
    }

    // Проверка, не в поездке ли уже
    alreadyIn, _ := s.tripRepo.IsPassengerInTrip(ctx, tripID, passengerID)
    if alreadyIn {
        return nil, model.ErrAlreadyInTrip
    }

    // Проверка конфликта расписания
    conflict, _ := s.tripRepo.HasSchedulingConflict(ctx, passengerID, trip.DepartAt, "passenger")
    if conflict {
        return nil, model.ErrSchedulingConflict
    }

    // --- Геометрические проверки (PostGIS) ---

    distToRoute, err := s.tripRepo.GetDistanceToRoute(ctx, tripID, req.PickupLat, req.PickupLng)
    if err != nil {
        return nil, err
    }
    if distToRoute > float64(trip.Zone.MaxDistanceMeters) {
        return nil, &model.ErrTooFarFromRoute{
            ActualMeters: int(distToRoute),
            MaxMeters:    trip.Zone.MaxDistanceMeters,
        }
    }

    // --- Расчёт крюка через OSRM ---

    office, _ := s.officeRepo.GetByID(ctx, trip.OfficeID)
    newRoute, err := s.routing.GetRoute(ctx,
        trip.OriginLat, trip.OriginLng,       // старт водителя
        req.PickupLat, req.PickupLng,          // точка пассажира
        office.Lat, office.Lng,                // офис
    )
    if err != nil {
        return nil, fmt.Errorf("routing detour calculation error: %w", err)
    }

    detourSeconds := newRoute.DurationSeconds - trip.DurationSeconds
    detourMinutes := detourSeconds / 60

    if detourMinutes > trip.Zone.MaxDetourMinutes {
        return nil, &model.ErrDetourExceeded{
            ActualMinutes: detourMinutes,
            MaxMinutes:    trip.Zone.MaxDetourMinutes,
        }
    }

    // --- Всё проверено, создаём запись ---

    passenger := &model.TripPassenger{
        ID:            uuid.New(),
        TripID:        tripID,
        UserID:        passengerID,
        PickupLat:     req.PickupLat,
        PickupLng:     req.PickupLng,
        PickupAddress: req.PickupAddress,
        DetourSeconds: detourSeconds,
        Status:        "confirmed",
    }

    if err := s.tripRepo.AddPassengerTx(ctx, tx, passenger); err != nil {
        return nil, err
    }

    // Уменьшаем счётчик мест
    if err := s.tripRepo.DecrementSeatsTx(ctx, tx, tripID); err != nil {
        return nil, err
    }

    if err := tx.Commit(); err != nil {
        return nil, err
    }

    // --- Уведомление водителю ---
    passUser, _ := s.userRepo.GetByID(ctx, passengerID)
    s.notif.NotifyUser(trip.DriverID, model.WSMessage{
        Type: "passenger_joined",
        Data: map[string]any{
            "trip_id":    tripID,
            "passenger":  passUser,
            "seats_left": trip.SeatsLeft - 1,
        },
    })

    return passenger, nil
}
```

### model/errors.go

```go
package model

import (
    "errors"
    "fmt"
)

// Sentinel ошибки
var (
    ErrTripNotFound       = errors.New("trip not found")
    ErrTripNotScheduled   = errors.New("trip is not in scheduled status")
    ErrNoSeatsAvailable   = errors.New("no seats available")
    ErrCannotJoinOwnTrip  = errors.New("driver cannot join own trip")
    ErrJoinTooLate        = errors.New("too late to join trip")
    ErrAlreadyInTrip      = errors.New("already in this trip")
    ErrSchedulingConflict = errors.New("scheduling conflict with another trip")
    ErrOriginOutsideZone  = errors.New("origin point is outside the allowed zone")
    ErrTooSoonToCreate    = errors.New("trip must be created at least 60 minutes before departure")
    ErrUserNotFound       = errors.New("user not found")
    ErrEmailDomainNotAllowed = errors.New("email domain not allowed for this company")
    ErrInvalidCredentials = errors.New("invalid email or password")
    ErrEmailNotVerified   = errors.New("email not verified")
)

// Ошибки с данными (для информативных сообщений пользователю)
type ErrTooFarFromRoute struct {
    ActualMeters int
    MaxMeters    int
}

func (e *ErrTooFarFromRoute) Error() string {
    return fmt.Sprintf("pickup point is %.1f km from route, maximum is %.1f km",
        float64(e.ActualMeters)/1000,
        float64(e.MaxMeters)/1000,
    )
}

type ErrDetourExceeded struct {
    ActualMinutes int
    MaxMinutes    int
}

func (e *ErrDetourExceeded) Error() string {
    return fmt.Sprintf("detour for driver would be %d min, maximum is %d min",
        e.ActualMinutes, e.MaxMinutes,
    )
}
```

### middleware/auth.go

```go
package middleware

import (
    "context"
    "net/http"
    "strings"

    "github.com/golang-jwt/jwt/v5"
)

type contextKey string
const (
    ContextUserID   contextKey = "user_id"
    ContextUserRole contextKey = "user_role"
)

func Auth(jwtSecret string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
                return
            }

            parts := strings.SplitN(authHeader, " ", 2)
            if len(parts) != 2 || parts[0] != "Bearer" {
                http.Error(w, `{"error":"invalid_token_format"}`, http.StatusUnauthorized)
                return
            }

            token, err := jwt.Parse(parts[1], func(t *jwt.Token) (any, error) {
                if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
                    return nil, jwt.ErrSignatureInvalid
                }
                return []byte(jwtSecret), nil
            })
            if err != nil || !token.Valid {
                http.Error(w, `{"error":"invalid_token"}`, http.StatusUnauthorized)
                return
            }

            claims, ok := token.Claims.(jwt.MapClaims)
            if !ok {
                http.Error(w, `{"error":"invalid_claims"}`, http.StatusUnauthorized)
                return
            }

            ctx := context.WithValue(r.Context(), ContextUserID, claims["sub"])
            ctx = context.WithValue(ctx, ContextUserRole, claims["role"])
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func RequireRole(roles ...string) func(http.Handler) http.Handler {
    allowed := make(map[string]bool)
    for _, r := range roles {
        allowed[r] = true
    }
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            role, _ := r.Context().Value(ContextUserRole).(string)
            if !allowed[role] {
                http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

### handler/trip_handler.go (фрагмент)

```go
package handler

import (
    "encoding/json"
    "errors"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "github.com/yourcompany/carpooling/internal/middleware"
    "github.com/yourcompany/carpooling/internal/model"
)

func (h *Handler) joinTrip(w http.ResponseWriter, r *http.Request) {
    tripIDStr := chi.URLParam(r, "id")
    tripID, err := uuid.Parse(tripIDStr)
    if err != nil {
        h.respondError(w, http.StatusBadRequest, "invalid_trip_id", "Неверный ID поездки")
        return
    }

    userID, _ := r.Context().Value(middleware.ContextUserID).(string)
    passengerID, _ := uuid.Parse(userID)

    var req model.JoinTripRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        h.respondError(w, http.StatusBadRequest, "invalid_body", "Неверный формат запроса")
        return
    }

    if err := h.validate.Struct(req); err != nil {
        h.respondError(w, http.StatusBadRequest, "validation_error", err.Error())
        return
    }

    passenger, err := h.tripSvc.JoinTrip(r.Context(), passengerID, tripID, req)
    if err != nil {
        // Маппинг ошибок на HTTP коды и сообщения
        switch {
        case errors.Is(err, model.ErrNoSeatsAvailable):
            h.respondError(w, http.StatusConflict, "no_seats_available", "Нет свободных мест")
        case errors.Is(err, model.ErrJoinTooLate):
            h.respondError(w, http.StatusBadRequest, "join_too_late", "Слишком поздно присоединяться")
        case errors.Is(err, model.ErrSchedulingConflict):
            h.respondError(w, http.StatusConflict, "scheduling_conflict", "Конфликт с другой поездкой")
        default:
            var errFar *model.ErrTooFarFromRoute
            var errDetour *model.ErrDetourExceeded
            switch {
            case errors.As(err, &errFar):
                h.respondError(w, http.StatusBadRequest, "too_far_from_route", err.Error())
            case errors.As(err, &errDetour):
                h.respondError(w, http.StatusBadRequest, "detour_exceeded", err.Error())
            default:
                h.respondError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка")
            }
        }
        return
    }

    h.respondJSON(w, http.StatusOK, map[string]any{
        "message":        "Вы присоединились к поездке",
        "detour_seconds": passenger.DetourSeconds,
    })
}

// Вспомогательные методы
func (h *Handler) respondJSON(w http.ResponseWriter, status int, data any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(data)
}

func (h *Handler) respondError(w http.ResponseWriter, status int, code, message string) {
    h.respondJSON(w, status, map[string]string{
        "error":   code,
        "message": message,
    })
}
```

### service/routing_service.go

```go
package service

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
)

type RoutingService struct {
    baseURL    string
    httpClient *http.Client
}

type RouteResult struct {
    GeoJSON         string
    DurationSeconds int
    DistanceMeters  int
}

// GetRoute строит маршрут через несколько точек
// points: пары lat,lng — минимум 2 точки (старт и финиш)
func (s *RoutingService) GetRoute(ctx context.Context, points ...float64) (*RouteResult, error) {
    if len(points)%2 != 0 || len(points) < 4 {
        return nil, fmt.Errorf("invalid points: need pairs of lat,lng")
    }

    // OSRM принимает координаты в формате lng,lat
    coords := make([]string, 0, len(points)/2)
    for i := 0; i < len(points); i += 2 {
        lat, lng := points[i], points[i+1]
        coords = append(coords, fmt.Sprintf("%.6f,%.6f", lng, lat))
    }

    url := fmt.Sprintf("%s/route/v1/driving/%s?overview=full&geometries=geojson",
        s.baseURL,
        strings.Join(coords, ";"),
    )

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return nil, err
    }

    resp, err := s.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("OSRM request failed: %w", err)
    }
    defer resp.Body.Close()

    var osrmResp struct {
        Code   string `json:"code"`
        Routes []struct {
            Duration float64 `json:"duration"` // секунды
            Distance float64 `json:"distance"` // метры
            Geometry struct {
                Type        string      `json:"type"`
                Coordinates [][]float64 `json:"coordinates"`
            } `json:"geometry"`
        } `json:"routes"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&osrmResp); err != nil {
        return nil, fmt.Errorf("OSRM response decode error: %w", err)
    }

    if osrmResp.Code != "Ok" || len(osrmResp.Routes) == 0 {
        return nil, fmt.Errorf("OSRM returned no routes, code: %s", osrmResp.Code)
    }

    route := osrmResp.Routes[0]
    geojsonBytes, _ := json.Marshal(route.Geometry)

    return &RouteResult{
        GeoJSON:         string(geojsonBytes),
        DurationSeconds: int(route.Duration),
        DistanceMeters:  int(route.Distance),
    }, nil
}
```

---

## 12. Фронтенд

### Базовый layout (web/templates/layout/base.html)

```html
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{block "title" .}}Поездки в офис{{end}}</title>

    <!-- Tailwind CSS -->
    <script src="https://cdn.tailwindcss.com"></script>

    <!-- HTMX -->
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>

    <!-- Alpine.js -->
    <script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.13.0/dist/cdn.min.js"></script>

    <!-- Leaflet -->
    <link rel="stylesheet" href="https://unpkg.com/leaflet@1.9.4/dist/leaflet.css"/>
    <script src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js"></script>

    {{block "head" .}}{{end}}
</head>
<body class="bg-gray-50 text-gray-900 min-h-screen"
      x-data="{ notification: null }"
      @notify.window="notification = $event.detail; setTimeout(() => notification = null, 4000)">

    {{template "nav.html" .}}

    <!-- Toast уведомление -->
    <div x-show="notification"
         x-transition
         class="fixed top-4 right-4 z-50 max-w-sm bg-white shadow-lg rounded-lg p-4 border-l-4"
         :class="notification?.type === 'error' ? 'border-red-500' : 'border-green-500'">
        <p x-text="notification?.message" class="text-sm"></p>
    </div>

    <main class="max-w-7xl mx-auto px-4 py-8">
        {{block "content" .}}{{end}}
    </main>

    {{block "scripts" .}}{{end}}
</body>
</html>
```

### Форма создания поездки (web/templates/trips/create.html)

```html
{{template "base.html" .}}

{{define "title"}}Создать поездку{{end}}

{{define "content"}}
<div class="grid grid-cols-1 lg:grid-cols-2 gap-8">

    <!-- Форма -->
    <div class="bg-white rounded-xl shadow p-6">
        <h1 class="text-2xl font-semibold mb-6">Создать поездку</h1>

        <form hx-post="/trips"
              hx-target="#result"
              hx-swap="innerHTML"
              x-data="tripForm()"
              class="space-y-4">

            <!-- Офис -->
            <div>
                <label class="block text-sm font-medium mb-1">Офис назначения</label>
                <select name="office_id"
                        x-model="officeId"
                        @change="loadZone()"
                        class="w-full border rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500">
                    <option value="">— выберите офис —</option>
                    {{range .Offices}}
                    <option value="{{.ID}}">{{.Name}}</option>
                    {{end}}
                </select>
            </div>

            <!-- Точка старта -->
            <div>
                <label class="block text-sm font-medium mb-1">Точка отправления</label>
                <p class="text-xs text-gray-500 mb-2">Нажмите на карту, чтобы выбрать точку</p>
                <input type="hidden" name="origin_lat" x-model="originLat" required>
                <input type="hidden" name="origin_lng" x-model="originLng" required>
                <input type="text"
                       name="origin_address"
                       x-model="originAddress"
                       placeholder="Адрес определится автоматически"
                       readonly
                       class="w-full border rounded-lg px-3 py-2 bg-gray-50 text-sm">
            </div>

            <!-- Дата и время -->
            <div>
                <label class="block text-sm font-medium mb-1">Дата и время выезда</label>
                <input type="datetime-local"
                       name="depart_at"
                       class="w-full border rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500">
            </div>

            <!-- Места -->
            <div>
                <label class="block text-sm font-medium mb-1">Количество мест для пассажиров</label>
                <select name="seats_total"
                        class="w-full border rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500">
                    <option value="1">1 место</option>
                    <option value="2" selected>2 места</option>
                    <option value="3">3 места</option>
                    <option value="4">4 места</option>
                </select>
            </div>

            <button type="submit"
                    :disabled="!originLat || !officeId"
                    class="w-full bg-blue-600 text-white py-2 rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed font-medium">
                Создать поездку
            </button>
        </form>

        <div id="result" class="mt-4"></div>
    </div>

    <!-- Карта -->
    <div>
        <div id="map" class="h-96 lg:h-full min-h-80 rounded-xl shadow"></div>
    </div>

</div>
{{end}}

{{define "scripts"}}
<script>
function tripForm() {
    return {
        officeId: '',
        originLat: '',
        originLng: '',
        originAddress: '',

        loadZone() {
            if (!this.officeId) return;
            // Загрузить зону офиса и показать на карте
            fetch(`/offices/${this.officeId}`)
                .then(r => r.json())
                .then(data => {
                    window.mapController.showOfficeZone(data);
                });
        }
    }
}

// Инициализация карты
document.addEventListener('DOMContentLoaded', () => {
    const map = L.map('map').setView([55.75, 37.62], 11);

    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
        attribution: '© OpenStreetMap contributors'
    }).addTo(map);

    let marker = null;
    let routeLayer = null;
    let zoneLayer = null;

    // Клик — ставим маркер старта
    map.on('click', async (e) => {
        const { lat, lng } = e.latlng;

        // Обновляем скрытые поля через Alpine
        const form = document.querySelector('[x-data]').__x.$data;
        form.originLat = lat.toFixed(6);
        form.originLng = lng.toFixed(6);

        // Реверс-геокодинг через Nominatim (бесплатно)
        const resp = await fetch(`https://nominatim.openstreetmap.org/reverse?lat=${lat}&lon=${lng}&format=json`);
        const geo = await resp.json();
        form.originAddress = geo.display_name || `${lat.toFixed(4)}, ${lng.toFixed(4)}`;

        // Маркер на карте
        if (marker) marker.remove();
        marker = L.marker([lat, lng], {
            icon: L.divIcon({ className: 'bg-green-500 w-4 h-4 rounded-full border-2 border-white shadow' })
        }).addTo(map);

        // Если офис выбран — строим маршрут превью
        if (form.officeId) {
            const route = await fetch(`/api/route-preview?origin_lat=${lat}&origin_lng=${lng}&office_id=${form.officeId}`);
            const routeData = await route.json();
            if (routeLayer) routeLayer.remove();
            routeLayer = L.geoJSON(JSON.parse(routeData.route_geojson), {
                style: { color: '#3b82f6', weight: 4, opacity: 0.8 }
            }).addTo(map);
        }
    });

    window.mapController = {
        showOfficeZone(officeData) {
            if (zoneLayer) zoneLayer.remove();
            if (officeData.zone_geojson) {
                zoneLayer = L.geoJSON(JSON.parse(officeData.zone_geojson), {
                    style: { color: '#10b981', fillOpacity: 0.1, dashArray: '6' }
                }).addTo(map);
                map.fitBounds(zoneLayer.getBounds(), { padding: [40, 40] });
            }
            // Маркер офиса
            L.marker([officeData.lat, officeData.lng], {
                icon: L.divIcon({ className: 'bg-red-500 w-4 h-4 rounded-full border-2 border-white shadow' })
            }).bindPopup(officeData.name).addTo(map);
        }
    };
});
</script>
{{end}}
```

---

## 13. Интеграция с картами (OSRM + Leaflet)

### OSRM — публичный бесплатный API

Базовый URL: `http://router.project-osrm.org`

```
# Маршрут от A до B
GET /route/v1/driving/{lng_a},{lat_a};{lng_b},{lat_b}?overview=full&geometries=geojson

# Маршрут через точку пассажира (A → пассажир → B)
GET /route/v1/driving/{lng_a},{lat_a};{lng_p},{lat_p};{lng_b},{lat_b}?overview=full&geometries=geojson
```

> **Важно:** публичный OSRM имеет rate limits. Для продакшна развернуть свой инстанс в Docker или использовать платный OSRM сервис / Google Maps Routes API.

### Альтернативные API карт

| Сервис | Бесплатный лимит | Маршруты | Геокодинг |
|---|---|---|---|
| OSRM (self-hosted) | Без лимитов | ✓ | — |
| OpenRouteService | 500 req/day | ✓ | ✓ |
| Google Maps Platform | $200 кредит/мес | ✓ | ✓ |
| Mapbox | 100k req/мес | ✓ | ✓ |
| Yandex Maps API | Платно | ✓ | ✓ |

### Геокодинг (адрес ↔ координаты)

```javascript
// Nominatim (бесплатно, требует User-Agent заголовок)
const reverseGeocode = async (lat, lng) => {
    const resp = await fetch(
        `https://nominatim.openstreetmap.org/reverse?lat=${lat}&lon=${lng}&format=json`,
        { headers: { 'User-Agent': 'CarpoolingApp/1.0' } }
    );
    const data = await resp.json();
    return data.display_name;
};

const forwardGeocode = async (query) => {
    const resp = await fetch(
        `https://nominatim.openstreetmap.org/search?q=${encodeURIComponent(query)}&format=json&limit=5`,
        { headers: { 'User-Agent': 'CarpoolingApp/1.0' } }
    );
    return resp.json();
};
```

---

## 14. WebSocket уведомления

### websocket/hub.go

```go
package websocket

import (
    "encoding/json"
    "log/slog"
    "sync"

    "github.com/google/uuid"
    ws "github.com/gorilla/websocket"
)

type Message struct {
    Type string `json:"type"`
    Data any    `json:"data"`
}

type Client struct {
    UserID uuid.UUID
    Conn   *ws.Conn
    Send   chan []byte
}

type Hub struct {
    clients    map[uuid.UUID]*Client
    register   chan *Client
    unregister chan *Client
    broadcast  chan *TargetedMessage
    mu         sync.RWMutex
}

type TargetedMessage struct {
    UserID  uuid.UUID
    Message Message
}

func NewHub() *Hub {
    return &Hub{
        clients:    make(map[uuid.UUID]*Client),
        register:   make(chan *Client),
        unregister: make(chan *Client),
        broadcast:  make(chan *TargetedMessage, 256),
    }
}

func (h *Hub) Run() {
    for {
        select {
        case client := <-h.register:
            h.mu.Lock()
            h.clients[client.UserID] = client
            h.mu.Unlock()
            slog.Info("ws client connected", "user_id", client.UserID)

        case client := <-h.unregister:
            h.mu.Lock()
            if _, ok := h.clients[client.UserID]; ok {
                delete(h.clients, client.UserID)
                close(client.Send)
            }
            h.mu.Unlock()
            slog.Info("ws client disconnected", "user_id", client.UserID)

        case msg := <-h.broadcast:
            h.mu.RLock()
            client, ok := h.clients[msg.UserID]
            h.mu.RUnlock()
            if !ok {
                continue
            }
            data, err := json.Marshal(msg.Message)
            if err != nil {
                continue
            }
            select {
            case client.Send <- data:
            default:
                // буфер переполнен — отключаем клиента
                h.mu.Lock()
                delete(h.clients, client.UserID)
                close(client.Send)
                h.mu.Unlock()
            }
        }
    }
}

func (h *Hub) SendToUser(userID uuid.UUID, msg Message) {
    h.broadcast <- &TargetedMessage{UserID: userID, Message: msg}
}

// writePump — горутина записи в WebSocket соединение
func (c *Client) WritePump() {
    defer c.Conn.Close()
    for msg := range c.Send {
        if err := c.Conn.WriteMessage(ws.TextMessage, msg); err != nil {
            slog.Warn("ws write error", "user_id", c.UserID, "error", err)
            return
        }
    }
}
```

### Подключение на фронте (Alpine.js)

```javascript
// В базовом layout, для авторизованных пользователей
document.addEventListener('DOMContentLoaded', () => {
    const token = document.querySelector('meta[name="access-token"]')?.content;
    if (!token) return;

    const ws = new WebSocket(`wss://${location.host}/ws?token=${token}`);

    ws.onmessage = (event) => {
        const msg = JSON.parse(event.data);

        switch (msg.type) {
            case 'passenger_joined':
                window.dispatchEvent(new CustomEvent('notify', {
                    detail: {
                        type: 'success',
                        message: `${msg.data.passenger.full_name} присоединился к вашей поездке`
                    }
                }));
                // Обновить счётчик мест через HTMX
                htmx.trigger(document.getElementById(`trip-${msg.data.trip_id}`), 'refresh');
                break;

            case 'trip_cancelled':
                window.dispatchEvent(new CustomEvent('notify', {
                    detail: { type: 'error', message: 'Водитель отменил поездку' }
                }));
                break;

            case 'passenger_left':
                window.dispatchEvent(new CustomEvent('notify', {
                    detail: { type: 'info', message: 'Один из пассажиров покинул поездку' }
                }));
                break;
        }
    };

    ws.onclose = () => {
        // Переподключение через 5 секунд
        setTimeout(() => location.reload(), 5000);
    };
});
```

---

## 15. Поэтапный план разработки

### Этап 0 — Инициализация (День 1–2)

**Цель:** рабочее окружение, пустой сервер, БД.

```bash
# Инициализация Go модуля
go mod init github.com/yourcompany/carpooling

# Зависимости
go get github.com/go-chi/chi/v5
go get github.com/jmoiron/sqlx
go get github.com/lib/pq
go get github.com/google/uuid
go get github.com/golang-jwt/jwt/v5
go get github.com/gorilla/websocket
go get github.com/go-playground/validator/v10
go get github.com/pressly/goose/v3
go get golang.org/x/crypto
```

**Задачи:**
- [ ] Создать структуру папок
- [ ] Написать `docker-compose.yml` с PostgreSQL + PostGIS
- [ ] Написать `main.go` с bare HTTP сервером
- [ ] Настроить `goose`, накатить первые 2 миграции
- [ ] Endpoint `GET /health` возвращает `{"status":"ok"}`
- [ ] `Makefile` с командами: `make run`, `make migrate`, `make build`

**Проверка:** `curl localhost:8080/health` возвращает 200 OK.

---

### Этап 1 — Авторизация (День 3–7)

**Цель:** регистрация, логин, JWT, верификация email.

**Задачи:**
- [ ] Миграции для `companies` и `users`
- [ ] `AuthService`: хэширование пароля (bcrypt), генерация JWT
- [ ] `POST /auth/register` — проверка домена, создание пользователя, отправка verify email
- [ ] `GET /auth/verify?token=xxx` — верификация email, смена роли на `passenger`
- [ ] `POST /auth/login` — проверка пароля, выдача токенов
- [ ] `POST /auth/refresh` — обновление access token через httpOnly cookie
- [ ] `POST /auth/logout` — инвалидация refresh token
- [ ] Auth middleware с проверкой JWT
- [ ] Role middleware
- [ ] Страницы: `/login`, `/register` (HTML шаблоны)
- [ ] Начальная seed-миграция: создать тестовую компанию и admin-пользователя

> **Упрощение на старте:** email верификацию можно заглушить (сразу ставить `email_verified=true`) и добавить нормальную позже.

**Проверка:** регистрация → логин → получение access token → запрос с токеном к protected endpoint.

---

### Этап 2 — Офисы (День 8–11)

**Цель:** CRUD офисов, зоны подбора, карта.

**Задачи:**
- [ ] Миграции `offices`, `office_zones`
- [ ] `OfficeService` и `OfficeRepository`
- [ ] `GET /offices` — список офисов (JSON + шаблон)
- [ ] `GET /offices/:id` — детали + зона (GeoJSON)
- [ ] Панель администратора: `/admin/offices`
- [ ] `POST /admin/offices` — создать офис
- [ ] `POST /admin/offices/:id/zones` — создать зону (принимает GeoJSON полигон)
- [ ] Главная страница с картой: все офисы как маркеры на Leaflet
- [ ] В панели админа: рисование зоны на карте через Leaflet.draw плагин

**Проверка:** admin создаёт офис → видит его на карте → создаёт зону подбора.

---

### Этап 3 — Создание поездок (День 12–17)

**Цель:** водитель создаёт поездку с маршрутом от OSRM.

**Задачи:**
- [ ] Миграция `trips`
- [ ] `RoutingService` — запросы к OSRM
- [ ] `TripService.CreateTrip` — полная бизнес-логика
- [ ] `TripRepository` — create, getByID, list
- [ ] `POST /trips` — API endpoint
- [ ] `GET /api/route-preview` — превью маршрута без сохранения (для карты)
- [ ] Страница `/trips/new` с картой и формой
- [ ] Страница `/trips/my` — список моих поездок
- [ ] `GET /trips/:id` — детальная страница с маршрутом на карте

**Проверка:** водитель кликает на карте → видит маршрут → создаёт поездку → видит её в списке.

---

### Этап 4 — Присоединение к поездке (День 18–24)

**Цель:** пассажир ищет и присоединяется к поездке, система проверяет ограничения.

**Задачи:**
- [ ] Миграция `trip_passengers`
- [ ] `TripService.JoinTrip` — полная логика с транзакцией
- [ ] `TripService.LeaveTrip` — отмена участия
- [ ] `TripRepository.GetDistanceToRoute` (PostGIS запрос)
- [ ] `TripRepository.HasSchedulingConflict`
- [ ] `GET /trips?office_id=&date=&lat=&lng=` — поиск с фильтрацией
- [ ] `POST /trips/:id/join` — API endpoint
- [ ] `POST /trips/:id/leave` — API endpoint
- [ ] Страница `/trips` — список с картой превью маршрутов
- [ ] Форма выбора точки подбора на карте (аналогично созданию)
- [ ] Информативные сообщения об ошибках валидации

**Проверка:** пассажир видит список поездок → выбирает точку → система отклоняет если далеко → принимает если всё ок.

---

### Этап 5 — WebSocket уведомления (День 25–28)

**Цель:** real-time уведомления при изменениях поездки.

**Задачи:**
- [ ] `websocket/hub.go`
- [ ] `GET /ws` handler с аутентификацией
- [ ] `NotificationService`
- [ ] Уведомления водителю: новый пассажир, пассажир ушёл
- [ ] Уведомления пассажирам: поездка отменена, изменено время
- [ ] Toast UI на фронте через Alpine.js
- [ ] Автоматическое обновление списка мест через HTMX при получении WS события
- [ ] `POST /trips/:id/cancel` — отмена поездки водителем с уведомлением всех пассажиров

**Проверка:** пассажир присоединяется → водитель видит уведомление без перезагрузки страницы.

---

### Этап 6 — Полировка и деплой (День 29–35)

**Задачи:**
- [ ] Пагинация в `/trips` и `/trips/my`
- [ ] Rate limiting на auth эндпоинты
- [ ] Обработка edge cases: водитель отменяет → пассажиры уведомлены, счётчик сброшен
- [ ] Страница `/admin/users` — список, изменение ролей
- [ ] Страница `/profile` — данные пользователя, история поездок
- [ ] Валидация всех форм на сервере (validate.Struct) и клиентская подсветка ошибок
- [ ] `Dockerfile` multi-stage build
- [ ] Caddy конфигурация
- [ ] `docker-compose.prod.yml`
- [ ] Деплой на Railway/Render

---

## 16. Docker и деплой

### docker-compose.yml (разработка)

```yaml
version: '3.8'

services:
  db:
    image: postgis/postgis:16-3.4
    environment:
      POSTGRES_DB: carpooling
      POSTGRES_USER: carpooling
      POSTGRES_PASSWORD: secret
    volumes:
      - pgdata:/var/lib/postgresql/data
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U carpooling"]
      interval: 5s
      timeout: 5s
      retries: 5

  app:
    build:
      context: .
      target: development  # для live-reload
    environment:
      DATABASE_URL: postgres://carpooling:secret@db:5432/carpooling?sslmode=disable
      PORT: 8080
      JWT_SECRET: dev-secret-change-in-prod
      OSRM_BASE_URL: http://router.project-osrm.org
    ports:
      - "8080:8080"
    volumes:
      - .:/app
    depends_on:
      db:
        condition: service_healthy

volumes:
  pgdata:
```

### Dockerfile

```dockerfile
# Стадия сборки
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./cmd/server

# Финальный образ
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/server .
COPY --from=builder /app/web ./web
COPY --from=builder /app/migrations ./migrations
EXPOSE 8080
CMD ["./server"]
```

### Makefile

```makefile
.PHONY: run build migrate migrate-down test lint docker-up docker-down

# Запуск в разработке
run:
	go run ./cmd/server

# Сборка бинаря
build:
	go build -o bin/server ./cmd/server

# Миграции вверх
migrate:
	goose -dir migrations postgres "$(DATABASE_URL)" up

# Миграции вниз (одна)
migrate-down:
	goose -dir migrations postgres "$(DATABASE_URL)" down

# Тесты
test:
	go test ./... -v

# Линтер
lint:
	golangci-lint run ./...

# Docker для разработки
docker-up:
	docker compose up -d

docker-down:
	docker compose down

# Создать seed данные (тестовая компания + admin)
seed:
	go run ./cmd/seed
```

---

## 17. Переменные окружения

```bash
# .env.example

# Сервер
PORT=8080
ENVIRONMENT=development  # development | production

# База данных
DATABASE_URL=postgres://carpooling:secret@localhost:5432/carpooling?sslmode=disable

# JWT
JWT_SECRET=your-secret-key-min-32-chars
JWT_ACCESS_EXPIRY=15m
JWT_REFRESH_EXPIRY=168h  # 7 дней

# OSRM (роутинг)
OSRM_BASE_URL=http://router.project-osrm.org
# Для продакшна лучше свой: http://your-osrm-server:5000

# Email (для верификации)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your@email.com
SMTP_PASSWORD=app-password
SMTP_FROM=noreply@yourcompany.com

# Приложение
APP_BASE_URL=https://carpooling.yourcompany.com
APP_NAME=Поездки в офис

# Глобальные дефолты ограничений (можно переопределить на уровне зоны)
DEFAULT_MAX_DETOUR_MINUTES=15
DEFAULT_MAX_DISTANCE_METERS=2000
DEFAULT_MIN_JOIN_MINUTES=30
```

---

## 18. Чеклист разработчика

### Перед началом каждой фичи
- [ ] Написаны миграции (если нужна новая таблица/поле)
- [ ] Определены модели в `/internal/model`
- [ ] Написаны интерфейсы репозитория
- [ ] SQL запросы проверены вручную в psql

### Перед PR/коммитом
- [ ] `go vet ./...` — ноль предупреждений
- [ ] `golangci-lint run` — ноль ошибок
- [ ] Все SQL запросы параметризованы (нет конкатенации строк)
- [ ] Все ошибки обрабатываются (нет `_` для error)
- [ ] Добавлено логирование в ключевых местах

### Безопасность (обязательно перед деплоем)
- [ ] Пароли — только bcrypt, нигде не логируются
- [ ] JWT secret — минимум 32 символа, из env
- [ ] Refresh token в httpOnly cookie с Secure флагом
- [ ] Rate limiting на /auth/login
- [ ] CORS только для разрешённых доменов
- [ ] Все input данные валидируются через validator
- [ ] Проверка принадлежности: пользователь может управлять только своими поездками

### Тестовые сценарии (ручное QA)

| Сценарий | Ожидаемый результат |
|---|---|
| Регистрация с чужим доменом | Ошибка "домен не разрешён" |
| Водитель ставит точку вне зоны | Ошибка "точка вне зоны" |
| Пассажир далеко от маршрута | Ошибка с указанием расстояния |
| Крюк больше лимита | Ошибка с указанием минут |
| Последнее место — два запроса одновременно | Только один успех (транзакция) |
| Отмена поездки водителем | Все пассажиры получают уведомление |
| Пассажир пытается уйти за < 30 мин | Ошибка "слишком поздно" |
| Создание двух поездок в одно время | Ошибка "конфликт расписания" |

---

*Документация актуальна для Go 1.22+, PostgreSQL 16, PostGIS 3.4*

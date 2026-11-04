# PageStream — Backend (Recommendation System)

REST API для системи персональних рекомендацій **PageStream** — сервіс рекомендацій фільмів та книг із векторним пошуком та соціальними функціями.

## Технологічний стек

| Категорія         | Технологія                              |
|-------------------|-----------------------------------------|
| Мова / Runtime    | Go 1.24                                 |
| HTTP Framework    | Gin                                     |
| База даних        | PostgreSQL + pgvector (вектори ембедингів) |
| ORM / Запити      | pgx/v5 (нативний PostgreSQL драйвер)    |
| Автентифікація    | JWT (golang-jwt/v5)                     |
| Міграції          | golang-migrate/v4                       |
| API документація  | Swagger (swaggo)                        |
| Зовнішні API      | TMDB (The Movie Database)               |
| Векторні ембединги | pgvector-go                            |

## Архітектура

```
cmd/
├── api/                   # Точка входу — HTTP сервер
├── generate_embeddings/   # CLI для генерації векторних ембедингів
├── import_movies/         # CLI для імпорту фільмів з TMDB
├── clear_movies/          # CLI для очищення бази фільмів
├── migrate/               # CLI для запуску міграцій
└── repair_movie_images/   # CLI для відновлення постерів фільмів

internal/
├── auth/            # Автентифікація (login, register, JWT)
├── books/           # Модуль книг (handlers, service, repository)
├── config/          # Конфігурація (env змінні)
├── database/        # Підключення до PostgreSQL
├── embeddings/      # Генерація та зберігання векторних ембедингів
├── friends/         # Соціальні функції (запити в друзі, список друзів)
├── importer/        # Імпорт контенту з зовнішніх API
├── library/         # Бібліотека користувача (favorites, watched, liked)
├── middleware/      # JWT middleware, CORS
├── models/          # Моделі даних
├── movies/          # Модуль фільмів (handlers, service, repository)
├── recommendations/ # Движок рекомендацій (векторна подібність + друзі)
├── repository/      # Загальні репозиторії (interactions, users)
├── search/          # Повнотекстовий та векторний пошук
├── security/        # Хешування паролів
├── tmdb/            # Клієнт TMDB API
└── users/           # Профілі користувачів
```

## Алгоритм рекомендацій

Система використовує гібридний підхід:

1. **Векторна подібність** — ембединги контенту порівнюються з профільним вектором користувача через `pgvector` cosine similarity
2. **Бустинг жанрів** — підвищення рейтингу для контенту з улюблених жанрів
3. **Соціальні рекомендації** — рекомендації на основі взаємодій друзів
4. **Re-ranking з диверсифікацією** — уникнення одноманітних результатів
5. **Виключення переглянутого** — контент, відзначений як `disliked` або `skipped`, виключається з рекомендацій

## Встановлення та запуск

### Вимоги

- Go 1.24+
- PostgreSQL 15+ з розширенням `pgvector`

### 1. Клонувати репозиторій та встановити залежності

```bash
git clone https://github.com/Vergil-David/recommendation-system.git
cd recommendation-system
go mod download
```

### 2. Налаштувати змінні середовища

Створіть файл `.env` у кореневій папці:

```env
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=recommendation_db
JWT_SECRET=your_jwt_secret
TMDB_API_KEY=your_tmdb_api_key
PORT=8080
```

### 3. Запустити міграції

```bash
go run ./cmd/migrate
```

### 4. Запустити сервер

```bash
go run ./cmd/api
```

API буде доступний за адресою `http://localhost:8080`.  
Swagger документація: `http://localhost:8080/swagger/index.html`

## CLI інструменти

| Команда                             | Опис                                      |
|-------------------------------------|-------------------------------------------|
| `go run ./cmd/api`                  | Запустити HTTP API сервер                 |
| `go run ./cmd/migrate`              | Застосувати міграції бази даних           |
| `go run ./cmd/import_movies`        | Імпортувати фільми з TMDB API             |
| `go run ./cmd/generate_embeddings`  | Згенерувати векторні ембединги для контенту |
| `go run ./cmd/clear_movies`         | Очистити базу фільмів                     |
| `go run ./cmd/repair_movie_images`  | Відновити відсутні постери фільмів        |

## API Ендпоінти (основні)

| Метод  | Шлях                         | Опис                              |
|--------|------------------------------|-----------------------------------|
| POST   | `/auth/register`             | Реєстрація                        |
| POST   | `/auth/login`                | Вхід (отримання JWT)              |
| GET    | `/movies`                    | Список фільмів                    |
| GET    | `/movies/:id`                | Деталі фільму                     |
| GET    | `/recommendations/movies`    | Рекомендовані фільми              |
| GET    | `/recommendations/books`     | Рекомендовані книги               |
| GET    | `/books`                     | Список книг                       |
| GET    | `/library`                   | Бібліотека користувача            |
| GET    | `/friends`                   | Список друзів                     |
| POST   | `/friends/request`           | Надіслати запит у друзі           |
| GET    | `/search`                    | Пошук фільмів та книг             |

Повна документація доступна через Swagger UI.

## Фронтенд репозиторій

[Front-recommendation-system](https://github.com/Vergil-David/Front-recommendation-system)

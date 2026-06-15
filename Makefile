.PHONY: run build migrate migrate-down test lint docker-up docker-down tidy

# Запуск в разработке
run:
	go run ./cmd/server

# Сборка бинаря
build:
	go build -o bin/server ./cmd/server

# Скачать зависимости
tidy:
	go mod tidy

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
	go vet ./...

# Docker для разработки
docker-up:
	docker compose up -d

# Docker для разработки (остановка)
docker-down:
	docker compose down

# Миграции через Docker (когда app запущен в compose)
migrate-docker:
	docker compose exec app sh -c 'DATABASE_URL=$$DATABASE_URL goose -dir migrations postgres "$$DATABASE_URL" up'

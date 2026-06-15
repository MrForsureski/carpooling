package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"office_trip/internal/model"
)

func TestPostgres_Integration(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://carpooling:secret@localhost:5432/carpooling?sslmode=disable"
	}

	db, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		t.Skipf("Skipping integration test: database not available: %v", err)
		return
	}
	defer db.Close()

	ctx := context.Background()
	txManager := NewTransactionManager(db)
	tripRepo := NewTripRepo(db)

	companyID := uuid.New()
	driverID := uuid.New()
	officeID := uuid.New()

	// Начинаем транзакцию для проведения теста и автоматического отката в конце
	err = txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		// 1. Создаем компанию, пользователя и офис для тестовой структуры внешних ключей
		_, err := getDbRunner(txCtx, db).ExecContext(txCtx, `
			INSERT INTO companies (id, name, email_domain, is_active, created_at, updated_at)
			VALUES ($1, 'Test Company Integration', 'test-integration.com', true, NOW(), NOW())
		`, companyID)
		if err != nil {
			return err
		}

		_, err = getDbRunner(txCtx, db).ExecContext(txCtx, `
			INSERT INTO users (id, company_id, email, password_hash, full_name, role, email_verified, is_active, created_at, updated_at)
			VALUES ($1, $2, 'driver-integration@test.com', 'hash', 'Driver Integration', 'driver', true, true, NOW(), NOW())
		`, driverID, companyID)
		if err != nil {
			return err
		}

		_, err = getDbRunner(txCtx, db).ExecContext(txCtx, `
			INSERT INTO offices (id, company_id, name, address, location, is_active, created_at, updated_at)
			VALUES ($1, $2, 'Office Moscow Integration', 'Moscow', ST_SetSRID(ST_MakePoint(37.61, 55.75), 4326), true, NOW(), NOW())
		`, officeID, companyID)
		if err != nil {
			return err
		}

		// 2. Создаем поездку
		trip := &model.Trip{
			ID:              uuid.New(),
			DriverID:        driverID,
			OfficeID:        officeID,
			OriginLat:       55.75,
			OriginLng:       37.61,
			OriginAddress:   "Start Point",
			DepartAt:        time.Now().Add(2 * time.Hour),
			SeatsTotal:      4,
			SeatsLeft:       4,
			RouteGeoJSON:    `{"type":"LineString","coordinates":[[37.61,55.75],[37.62,55.76]]}`,
			DurationSeconds: 600,
			DistanceMeters:  5000,
			Status:          model.TripStatusScheduled,
		}

		err = tripRepo.Create(txCtx, trip)
		if err != nil {
			return fmt.Errorf("failed to create trip: %w", err)
		}

		// 3. Выполняем GetForUpdate
		dbTrip, err := tripRepo.GetForUpdate(txCtx, trip.ID)
		if err != nil {
			return fmt.Errorf("failed to get trip for update: %w", err)
		}
		if dbTrip.SeatsLeft != 4 {
			return fmt.Errorf("expected seats left to be 4, got %d", dbTrip.SeatsLeft)
		}

		// 4. Декрементируем места
		err = tripRepo.DecrementSeats(txCtx, trip.ID)
		if err != nil {
			return fmt.Errorf("failed to decrement seats: %w", err)
		}

		// Проверяем изменение
		updatedTrip, err := tripRepo.GetByID(txCtx, trip.ID)
		if err != nil {
			return fmt.Errorf("failed to get updated trip: %w", err)
		}
		if updatedTrip.SeatsLeft != 3 {
			return fmt.Errorf("expected seats left to be 3 after decrement, got %d", updatedTrip.SeatsLeft)
		}

		// Возвращаем ошибку для автоматического отката транзакции и очистки БД
		return errors.New("test_rollback")
	})

	if err == nil || err.Error() != "test_rollback" {
		t.Errorf("expected transaction to rollback with 'test_rollback' error, got: %v", err)
	}
}

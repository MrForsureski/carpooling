package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/jmoiron/sqlx"
	"office_trip/internal/config"
	"office_trip/internal/handler"
	"office_trip/internal/infrastructure/osrm"
	pgRepo "office_trip/internal/repository/postgres"
	"office_trip/internal/service"
)

func main() {
	// структурированное логирование через slog
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	//загрузка конфигурации из env переменных окружения
	cfg := config.Load()

	if cfg.JWTSecret == "" {
		slog.Error("JWT_SECRET is not set")
		os.Exit(1)
	}

	//подключение к базе данных
	db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	defer db.Close()

	slog.Info("connected to database")

	// репозитории
	userRepo := pgRepo.NewUserRepo(db)
	tripRepo := pgRepo.NewTripRepo(db)
	officeRepo := pgRepo.NewOfficeRepo(db)

	// сервисы
	txManager := pgRepo.NewTransactionManager(db)
	authSvc := service.NewAuthService(userRepo, cfg)
	routingSvc := osrm.NewClient(cfg.OSRMBaseURL)
	officeSvc := service.NewOfficeService(officeRepo)
	tripSvc := service.NewTripService(tripRepo, officeRepo, userRepo, routingSvc, txManager)

	//хендлеры и роутер
	h := handler.New(handler.Deps{
		AuthSvc:   authSvc,
		TripSvc:   tripSvc,
		OfficeSvc: officeSvc,
		Config:    cfg,
	})

	//Http сервер
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      h.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	//запуск сервера в горутине
	go func() {
		slog.Info("server starting", "port", cfg.Port, "env", cfg.Environment)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// graceful shutdown завершение работы
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

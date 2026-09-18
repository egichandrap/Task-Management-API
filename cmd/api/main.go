package main

import (
	"database/sql"
	"task-api/internal/config"
	"task-api/internal/database/migration"
	"task-api/internal/domain"
	"task-api/internal/handler"
	"task-api/internal/middleware"
	"task-api/internal/repository"
	"task-api/pkg/logger"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
)

func main() {
	cfg := config.Load()
	logger.InitLogger()

	// ── Run migrations using raw database/sql ──────────────────────
	rawDB, err := sql.Open("postgres", cfg.DBDSN)
	if err != nil {
		log.Fatal("Failed to open DB for migration:", err)
	}
	if err := rawDB.Ping(); err != nil {
		log.Fatal("Failed to ping DB:", err)
	}

	migrator := migration.NewMigrator(rawDB, logger.Log)
	for _, m := range migration.AllMigrations() {
		migrator.AddMigration(m)
	}
	if err := migrator.Up(); err != nil {
		log.Fatal("Migration failed:", err)
	}
	rawDB.Close()

	// ── Open GORM connection for the app ───────────────────────────
	db, err := gorm.Open(postgres.Open(cfg.DBDSN), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to db", err)
	}

	// AutoMigrate kept as safety net for any GORM-specific column tweaks
	db.AutoMigrate(&domain.User{}, &domain.Task{}, &domain.TaskLog{}, &domain.IdempotencyRecord{})

	userRepo := repository.NewUserRepository(db)
	taskRepo := repository.NewTaskRepository(db)

	authHandler := handler.NewAuthHandler(userRepo, cfg)
	taskHandler := handler.NewTaskHandler(taskRepo)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.Use(middleware.Logger())
	r.Use(middleware.ErrorHandler())

	api := r.Group("/api")
	{
		api.POST("/register", authHandler.Register)
		api.POST("/login", authHandler.Login)

		protected := api.Group("/")
		protected.Use(middleware.Auth(cfg))
		{
			protected.POST("/tasks", middleware.Idempotency(taskRepo), taskHandler.Create)
			protected.GET("/tasks", taskHandler.List)
			protected.GET("/tasks/:id", taskHandler.Detail)
			protected.PUT("/tasks/:id", taskHandler.Update)
			protected.DELETE("/tasks/:id", taskHandler.Delete)
			protected.POST("/tasks/:id/assign", taskHandler.Assign)
		}
	}

	logger.Log.Info("Starting server on :" + cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal("Server failed", err)
	}
}

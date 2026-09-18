package main

import (
	"database/sql"
	"log"

	"task-api/internal/config"
	"task-api/internal/database/migration"
	"task-api/internal/handler"
	"task-api/internal/middleware"
	"task-api/internal/repository"
	projectusecase "task-api/internal/usecase/project"
	taskusecase "task-api/internal/usecase/task"
	userusecase "task-api/internal/usecase/user"
	"task-api/pkg/logger"
	"task-api/pkg/utils"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
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
	// TranslateError maps DB constraint violations (e.g. unique username)
	// to gorm.ErrDuplicatedKey so the repository can translate them into
	// domain errors. The SQL migrations are the single source of schema truth.
	db, err := gorm.Open(postgres.Open(cfg.DBDSN), &gorm.Config{TranslateError: true})
	if err != nil {
		log.Fatal("Failed to connect to db", err)
	}

	// ── Dependency Injection ───────────────────────────────────────
	// 1. Repositories (infrastructure adapters implementing domain ports)
	userRepo := repository.NewUserRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	projectRepo := repository.NewProjectRepository(db)
	idemStore := repository.NewIdempotencyStore(db)

	// 2. Usecases (application layer, one service per bounded context)
	authUsecase := userusecase.New(userRepo, utils.JWTIssuer{Secret: cfg.JWTSecret}, logger.Log)
	taskUsecase := taskusecase.New(taskRepo, userRepo, projectRepo, logger.Log)
	projectUsecase := projectusecase.New(projectRepo, userRepo, logger.Log)

	// 3. Handlers (transport layer)
	authHandler := handler.NewAuthHandler(authUsecase)
	taskHandler := handler.NewTaskHandler(taskUsecase)
	projectHandler := handler.NewProjectHandler(projectUsecase)

	// 4. Router
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
			protected.POST("/tasks", middleware.Idempotency(idemStore), taskHandler.Create)
			protected.GET("/tasks", taskHandler.List)
			protected.GET("/tasks/:id", taskHandler.Detail)
			protected.PUT("/tasks/:id", taskHandler.Update)
			protected.DELETE("/tasks/:id", taskHandler.Delete)
			protected.POST("/tasks/:id/assign", taskHandler.Assign)

			protected.POST("/projects", projectHandler.Create)
			protected.GET("/projects", projectHandler.ListMine)
			protected.GET("/projects/:id", projectHandler.Detail)
			protected.POST("/projects/:id/members", projectHandler.AddMember)
			protected.GET("/projects/:id/members", projectHandler.ListMembers)
			protected.DELETE("/projects/:id/members/:userId", projectHandler.RemoveMember)
		}
	}

	logger.Log.Info("Starting server on :" + cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal("Server failed", err)
	}
}

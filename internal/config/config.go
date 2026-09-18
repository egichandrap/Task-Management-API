package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port      string
	DBDSN     string
	JWTSecret string
}

func Load() *Config {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("[CONFIG] .env file not found, using system environment variables")
	} else {
		log.Println("[CONFIG] Loaded configuration from .env file")
	}

	port := getEnv("APP_PORT", "8080")

	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "")
	dbPassword := getEnv("DB_PASSWORD", "")
	dbName := getEnv("DB_NAME", "")
	dbSSLMode := getEnv("DB_SSLMODE", "disable")
	dbTimezone := getEnv("DB_TIMEZONE", "Asia/Jakarta")

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		dbHost, dbUser, dbPassword, dbName, dbPort, dbSSLMode, dbTimezone,
	)

	// Allow full DSN override via DB_DSN env var
	if override := os.Getenv("DB_DSN"); override != "" {
		dsn = override
	}

	jwtSecret := getEnv("JWT_SECRET", "supersecretkey")

	log.Printf("[CONFIG] Server Port : %s", port)
	log.Printf("[CONFIG] DB Host     : %s", dbHost)
	log.Printf("[CONFIG] DB Port     : %s", dbPort)
	log.Printf("[CONFIG] DB Name     : %s", dbName)
	log.Printf("[CONFIG] DB User     : %s", dbUser)
	log.Printf("[CONFIG] DB SSLMode  : %s", dbSSLMode)

	return &Config{
		Port:      port,
		DBDSN:     dsn,
		JWTSecret: jwtSecret,
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

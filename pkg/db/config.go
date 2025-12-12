package db

import (
	"os"
	"strconv"
)

type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

func LoadPostgresConfig() (PostgresConfig, error) {
	port, _ := strconv.Atoi(os.Getenv("DB_PORT"))

	return PostgresConfig{
		Host:     os.Getenv("DB_HOST"),
		Port:     port,
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		DBName:   os.Getenv("DB_NAME"),
		SSLMode:  os.Getenv("DB_SSLMODE"),
	}, nil
}

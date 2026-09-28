package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr               string
	LogLevel               string
	ShutdownTimeout        time.Duration
	DatabaseURL            string
	DatabaseMaxConns       int32
	DatabaseMinConns       int32
	DatabaseMaxConnLifetime time.Duration
	DatabaseConnectTimeout time.Duration
	DatabaseQueryTimeout   time.Duration
}

func Load() (Config, error) {
	var cfg Config
	var err error

	cfg.HTTPAddr, err = required("HTTP_ADDR")
	if err != nil {
		return Config{}, err
	}

	cfg.LogLevel, err = required("LOG_LEVEL")
	if err != nil {
		return Config{}, err
	}

	cfg.ShutdownTimeout, err = requiredDuration("SHUTDOWN_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	cfg.DatabaseURL, err = required("DATABASE_URL")
	if err != nil {
		return Config{}, err
	}

	cfg.DatabaseMaxConns, err = requiredInt32("DATABASE_MAX_CONNS")
	if err != nil {
		return Config{}, err
	}

	cfg.DatabaseMinConns, err = requiredInt32("DATABASE_MIN_CONNS")
	if err != nil {
		return Config{}, err
	}

	cfg.DatabaseMaxConnLifetime, err = requiredDuration("DATABASE_MAX_CONN_LIFETIME")
	if err != nil {
		return Config{}, err
	}

	cfg.DatabaseConnectTimeout, err = requiredDuration("DATABASE_CONNECT_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	cfg.DatabaseQueryTimeout, err = requiredDuration("DATABASE_QUERY_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func required(name string) (string, error) {
	v := os.Getenv(name)
	if v == "" {
		return "", fmt.Errorf("required environment variable %s is not set", name)
	}
	return v, nil
}

func requiredDuration(name string) (time.Duration, error) {
	v, err := required(name)
	if err != nil {
		return 0, err
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}
	return d, nil
}

func requiredInt32(name string) (int32, error) {
	v, err := required(name)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}
	return int32(n), nil
}

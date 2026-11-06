package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr                string
	HTTPReadTimeout         time.Duration
	HTTPReadHeaderTimeout   time.Duration
	HTTPWriteTimeout        time.Duration
	HTTPIdleTimeout         time.Duration
	LogLevel                slog.Level
	ShutdownTimeout         time.Duration
	DatabaseURL             string
	DatabaseMaxConns        int32
	DatabaseMinConns        int32
	DatabaseMaxConnLifetime time.Duration
	DatabaseConnectTimeout  time.Duration
	DatabaseQueryTimeout    time.Duration
	OTelServiceName         string
	OTelExporterEndpoint    string
	OTelExporterProtocol    string
	OTelResourceAttributes  string
	OTelTracesSampler       string
}

func Load() (Config, error) {
	loaders := []func(*Config) error{loadHTTP, loadDatabase, loadTelemetry}
	var cfg Config
	for _, load := range loaders {
		if err := load(&cfg); err != nil {
			return Config{}, err
		}
	}
	return cfg, nil
}

func loadHTTP(cfg *Config) error {
	var err error
	if cfg.HTTPAddr, err = required("HTTP_ADDR"); err != nil {
		return err
	}
	if cfg.HTTPReadTimeout, err = optionalDuration("HTTP_READ_TIMEOUT", 10*time.Second); err != nil {
		return err
	}
	if cfg.HTTPReadHeaderTimeout, err = optionalDuration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second); err != nil {
		return err
	}
	if cfg.HTTPWriteTimeout, err = optionalDuration("HTTP_WRITE_TIMEOUT", 15*time.Second); err != nil {
		return err
	}
	if cfg.HTTPIdleTimeout, err = optionalDuration("HTTP_IDLE_TIMEOUT", 60*time.Second); err != nil {
		return err
	}
	if cfg.ShutdownTimeout, err = requiredDuration("SHUTDOWN_TIMEOUT"); err != nil {
		return err
	}
	level, err := required("LOG_LEVEL")
	if err != nil {
		return err
	}
	if err := cfg.LogLevel.UnmarshalText([]byte(level)); err != nil {
		return fmt.Errorf("parse LOG_LEVEL: %w", err)
	}
	return nil
}

func loadDatabase(cfg *Config) error {
	var err error
	if cfg.DatabaseURL, err = required("DATABASE_URL"); err != nil {
		return err
	}
	if cfg.DatabaseMaxConns, err = requiredInt32("DATABASE_MAX_CONNS"); err != nil {
		return err
	}
	if cfg.DatabaseMinConns, err = requiredInt32("DATABASE_MIN_CONNS"); err != nil {
		return err
	}
	if cfg.DatabaseMaxConnLifetime, err = requiredDuration("DATABASE_MAX_CONN_LIFETIME"); err != nil {
		return err
	}
	if cfg.DatabaseConnectTimeout, err = requiredDuration("DATABASE_CONNECT_TIMEOUT"); err != nil {
		return err
	}
	if cfg.DatabaseQueryTimeout, err = requiredDuration("DATABASE_QUERY_TIMEOUT"); err != nil {
		return err
	}
	return nil
}

func loadTelemetry(cfg *Config) error {
	var err error
	if cfg.OTelServiceName, err = required("OTEL_SERVICE_NAME"); err != nil {
		return err
	}
	if cfg.OTelExporterEndpoint, err = required("OTEL_EXPORTER_OTLP_ENDPOINT"); err != nil {
		return err
	}
	if cfg.OTelExporterProtocol, err = required("OTEL_EXPORTER_OTLP_PROTOCOL"); err != nil {
		return err
	}
	if cfg.OTelExporterProtocol != "http/protobuf" {
		return fmt.Errorf("OTEL_EXPORTER_OTLP_PROTOCOL must be http/protobuf")
	}
	if cfg.OTelResourceAttributes, err = required("OTEL_RESOURCE_ATTRIBUTES"); err != nil {
		return err
	}
	if cfg.OTelTracesSampler, err = required("OTEL_TRACES_SAMPLER"); err != nil {
		return err
	}
	return nil
}

func required(name string) (string, error) {
	value := os.Getenv(name)
	if value == "" {
		return "", fmt.Errorf("required environment variable %s is not set", name)
	}
	return value, nil
}

func requiredDuration(name string) (time.Duration, error) {
	value, err := required(name)
	if err != nil {
		return 0, err
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}
	return duration, nil
}

func optionalDuration(name string, defaultValue time.Duration) (time.Duration, error) {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}
	return duration, nil
}

func requiredInt32(name string) (int32, error) {
	value, err := required(name)
	if err != nil {
		return 0, err
	}
	number, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	if number <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}
	return int32(number), nil
}

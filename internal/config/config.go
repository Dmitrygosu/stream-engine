package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// MinJWTSecretLength is the minimum HMAC key size accepted for HS256 (RFC 7518, 3.2).
const MinJWTSecretLength = 32

type Config struct {
	App      AppConfig      `mapstructure:"APP"`
	HTTP     HTTPConfig     `mapstructure:"HTTP"`
	Postgres PostgresConfig `mapstructure:"POSTGRES"`
	Kafka    KafkaConfig    `mapstructure:"KAFKA"`
	Logger   LoggerConfig   `mapstructure:"LOGGER"`
	Auth     AuthConfig     `mapstructure:"AUTH"`
}

type AppConfig struct {
	Name    string `mapstructure:"NAME"`
	Version string `mapstructure:"VERSION"`
}

type HTTPConfig struct {
	Port string `mapstructure:"PORT"`
}

type PostgresConfig struct {
	URL string `mapstructure:"URL"`
}

type KafkaConfig struct {
	Brokers []string `mapstructure:"BROKERS"`
}

type LoggerConfig struct {
	Level string `mapstructure:"LEVEL"`
}

type AuthConfig struct {
	JWTSecret string        `mapstructure:"JWT_SECRET"`
	Audience  string        `mapstructure:"AUDIENCE"`
	TokenTTL  time.Duration `mapstructure:"TOKEN_TTL"`
}

// Load reads the configuration from the environment and an optional config.yaml.
func Load() (*Config, error) {
	viper.SetDefault("APP.NAME", "stream-engine")
	viper.SetDefault("APP.VERSION", "0.1.0")
	viper.SetDefault("HTTP.PORT", "8080")
	viper.SetDefault("LOGGER.LEVEL", "info")
	viper.SetDefault("AUTH.AUDIENCE", "stream-engine")
	viper.SetDefault("AUTH.TOKEN_TTL", "1h")

	viper.AutomaticEnv()

	envBindings := map[string]string{
		"POSTGRES.URL":    "DB_URL",
		"HTTP.PORT":       "HTTP_PORT",
		"KAFKA.BROKERS":   "KAFKA_BROKERS",
		"LOGGER.LEVEL":    "LOGGER_LEVEL",
		"AUTH.JWT_SECRET": "JWT_SECRET",
		"AUTH.AUDIENCE":   "JWT_AUDIENCE",
		"AUTH.TOKEN_TTL":  "JWT_TTL",
	}
	for key, env := range envBindings {
		if err := viper.BindEnv(key, env); err != nil {
			return nil, fmt.Errorf("config: bind %s: %w", env, err)
		}
	}

	viper.AddConfigPath(".")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	var notFound viper.ConfigFileNotFoundError
	if err := viper.ReadInConfig(); err != nil && !errors.As(err, &notFound) {
		return nil, fmt.Errorf("config: read config.yaml: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}
	return &cfg, nil
}

// Validate reports every missing or unsafe setting at once. The worker does
// not issue or check tokens, so it validates without the auth settings.
func (c *Config) Validate(requireAuth bool) error {
	var errs []error
	if c.Postgres.URL == "" {
		errs = append(errs, errors.New("DB_URL is required"))
	}
	if len(c.Kafka.Brokers) == 0 {
		errs = append(errs, errors.New("KAFKA_BROKERS is required"))
	}
	if requireAuth {
		if len(c.Auth.JWTSecret) < MinJWTSecretLength {
			errs = append(errs, fmt.Errorf("JWT_SECRET must be at least %d bytes", MinJWTSecretLength))
		}
		if c.Auth.Audience == "" {
			errs = append(errs, errors.New("JWT_AUDIENCE must not be empty"))
		}
		if c.Auth.TokenTTL <= 0 {
			errs = append(errs, errors.New("JWT_TTL must be positive"))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	return nil
}

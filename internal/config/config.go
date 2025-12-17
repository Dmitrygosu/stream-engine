package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig      `mapstructure:"APP"`
	HTTP     HTTPConfig     `mapstructure:"HTTP"`
	Postgres PostgresConfig `mapstructure:"POSTGRES"`
	Kafka    KafkaConfig    `mapstructure:"KAFKA"`
	Redis    RedisConfig    `mapstructure:"REDIS"`
	Logger   LoggerConfig   `mapstructure:"LOGGER"`
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

type RedisConfig struct {
	Addr string `mapstructure:"ADDR"`
}

type LoggerConfig struct {
	Level string `mapstructure:"LEVEL"`
}

func Load() (*Config, error) {
	viper.SetDefault("APP.NAME", "stream-engine")
	viper.SetDefault("APP.VERSION", "0.1.0")
	viper.SetDefault("HTTP.PORT", "8080")
	viper.SetDefault("LOGGER.LEVEL", "info")

	viper.AutomaticEnv()

	viper.BindEnv("POSTGRES.URL", "DB_URL")
	viper.BindEnv("HTTP.PORT", "HTTP_PORT")
	viper.BindEnv("KAFKA.BROKERS", "KAFKA_BROKERS")
	viper.BindEnv("REDIS.ADDR", "REDIS_ADDR")

	viper.AddConfigPath(".")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	_ = viper.ReadInConfig()

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}

	return &cfg, nil
}

package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Log      LogConfig      `mapstructure:"log"`
	JWT      JWTConfig      `mapstructure:"jwt"`
}

type ServerConfig struct {
	Port     string `mapstructure:"port"`
	GRPCPort string `mapstructure:"grpc_port"`
	Host     string `mapstructure:"host"`
}

type DatabaseConfig struct {
	URL     string `mapstructure:"url"`
	MaxConn int    `mapstructure:"max_connections"`
	MaxIdle int    `mapstructure:"max_idle"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type JWTConfig struct {
	SecretKey      string `mapstructure:"secret_key"`
	ExpirationTime int    `mapstructure:"expiration_time"`
}

func Load() *Config {
	// Set defaults
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.grpc_port", "9090")
	viper.SetDefault("server.host", "localhost")
	viper.SetDefault("database.url", "postgres://immich:immich@localhost:5432/immich?sslmode=disable")
	viper.SetDefault("database.max_connections", 100)
	viper.SetDefault("database.max_idle", 10)
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")
	viper.SetDefault("jwt.secret_key", "your-secret-key-change-this")
	viper.SetDefault("jwt.expiration_time", 3600) // 1 hour

	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		panic("Failed to unmarshal config: " + err.Error())
	}

	return cfg
}

package bootstrap

import "github.com/spf13/viper"

type Config struct {
	GRPCPort    int    `mapstructure:"GRPC_PORT"`
	RabbitMQURL string `mapstructure:"RABBITMQ_URL"`
	RedisAddr   string `mapstructure:"REDIS_ADDR"`
	APMServerURL string `mapstructure:"ELASTIC_APM_SERVER_URL"`
}

func LoadConfig() (*Config, error) {
	viper.AutomaticEnv()
	viper.SetDefault("GRPC_PORT", 50058)
	viper.SetDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	viper.SetDefault("REDIS_ADDR", "localhost:6379")

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

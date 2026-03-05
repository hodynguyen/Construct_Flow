package bootstrap

import "github.com/spf13/viper"

type Config struct {
	DBHost      string `mapstructure:"DB_HOST"`
	DBPort      int    `mapstructure:"DB_PORT"`
	DBName      string `mapstructure:"DB_NAME"`
	DBUser      string `mapstructure:"DB_USER"`
	DBPassword  string `mapstructure:"DB_PASSWORD"`
	RedisAddr   string `mapstructure:"REDIS_ADDR"`
	RabbitMQURL string `mapstructure:"RABBITMQ_URL"`
	GRPCPort    int    `mapstructure:"GRPC_PORT"`
	APMServerURL string `mapstructure:"ELASTIC_APM_SERVER_URL"`
}

func LoadConfig() (*Config, error) {
	viper.AutomaticEnv()
	viper.SetDefault("DB_HOST", "localhost")
	viper.SetDefault("DB_PORT", 5432)
	viper.SetDefault("DB_NAME", "constructflow")
	viper.SetDefault("DB_USER", "admin")
	viper.SetDefault("DB_PASSWORD", "secret")
	viper.SetDefault("REDIS_ADDR", "localhost:6379")
	viper.SetDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	viper.SetDefault("GRPC_PORT", 50052)

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

package bootstrap

import "github.com/spf13/viper"

type Config struct {
	DBHost       string `mapstructure:"DB_HOST"`
	DBPort       int    `mapstructure:"DB_PORT"`
	DBName       string `mapstructure:"DB_NAME"`
	DBUser       string `mapstructure:"DB_USER"`
	DBPassword   string `mapstructure:"DB_PASSWORD"`
	GRPCPort     int    `mapstructure:"GRPC_PORT"`
	RabbitMQURL  string `mapstructure:"RABBITMQ_URL"`
	APMServerURL string `mapstructure:"ELASTIC_APM_SERVER_URL"`

	// MinIO / S3 config
	S3Endpoint        string `mapstructure:"S3_ENDPOINT"`
	S3AccessKeyID     string `mapstructure:"S3_ACCESS_KEY_ID"`
	S3SecretAccessKey string `mapstructure:"S3_SECRET_ACCESS_KEY"`
	S3BucketName      string `mapstructure:"S3_BUCKET_NAME"`
	S3UseSSL          bool   `mapstructure:"S3_USE_SSL"`
}

func LoadConfig() (*Config, error) {
	viper.AutomaticEnv()
	viper.SetDefault("DB_HOST", "localhost")
	viper.SetDefault("DB_PORT", 5432)
	viper.SetDefault("DB_NAME", "constructflow")
	viper.SetDefault("DB_USER", "admin")
	viper.SetDefault("DB_PASSWORD", "secret")
	viper.SetDefault("GRPC_PORT", 50054)
	viper.SetDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	viper.SetDefault("S3_ENDPOINT", "localhost:9000")
	viper.SetDefault("S3_ACCESS_KEY_ID", "minioadmin")
	viper.SetDefault("S3_SECRET_ACCESS_KEY", "minioadmin")
	viper.SetDefault("S3_BUCKET_NAME", "constructflow")
	viper.SetDefault("S3_USE_SSL", false)

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

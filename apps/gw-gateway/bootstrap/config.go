package bootstrap

import "github.com/spf13/viper"

type Config struct {
	HTTPPort                   int    `mapstructure:"HTTP_PORT"`
	RedisAddr                  string `mapstructure:"REDIS_ADDR"`
	TaskServiceAddr            string `mapstructure:"TASK_SERVICE_ADDR"`
	UserServiceAddr            string `mapstructure:"USER_SERVICE_ADDR"`
	NotificationServiceAddr    string `mapstructure:"NOTIFICATION_SERVICE_ADDR"`
	JWTPublicKeyPath           string `mapstructure:"JWT_PUBLIC_KEY_PATH"`
	RateLimitRequestsPerMinute int    `mapstructure:"RATE_LIMIT_RPM"`
	CasbinModelPath            string `mapstructure:"CASBIN_MODEL_PATH"`
	CasbinPolicyPath           string `mapstructure:"CASBIN_POLICY_PATH"`
	APMServerURL               string `mapstructure:"ELASTIC_APM_SERVER_URL"`
}

func LoadConfig() (*Config, error) {
	viper.AutomaticEnv()
	viper.SetDefault("HTTP_PORT", 8080)
	viper.SetDefault("REDIS_ADDR", "localhost:6379")
	viper.SetDefault("TASK_SERVICE_ADDR", "localhost:50051")
	viper.SetDefault("USER_SERVICE_ADDR", "localhost:50053")
	viper.SetDefault("NOTIFICATION_SERVICE_ADDR", "localhost:50052")
	viper.SetDefault("JWT_PUBLIC_KEY_PATH", "./keys/public.pem")
	viper.SetDefault("RATE_LIMIT_RPM", 60)
	viper.SetDefault("CASBIN_MODEL_PATH", "./casbin/model.conf")
	viper.SetDefault("CASBIN_POLICY_PATH", "./casbin/policy.csv")

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

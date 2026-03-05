package bootstrap

import "github.com/spf13/viper"

type Config struct {
	DBHost          string `mapstructure:"DB_HOST"`
	DBPort          int    `mapstructure:"DB_PORT"`
	DBName          string `mapstructure:"DB_NAME"`
	DBUser          string `mapstructure:"DB_USER"`
	DBPassword      string `mapstructure:"DB_PASSWORD"`
	GRPCPort        int    `mapstructure:"GRPC_PORT"`
	JWTPrivateKeyPath string `mapstructure:"JWT_PRIVATE_KEY_PATH"`
	JWTPublicKeyPath  string `mapstructure:"JWT_PUBLIC_KEY_PATH"`
	APMServerURL    string `mapstructure:"ELASTIC_APM_SERVER_URL"`
}

func LoadConfig() (*Config, error) {
	viper.AutomaticEnv()
	viper.SetDefault("DB_HOST", "localhost")
	viper.SetDefault("DB_PORT", 5432)
	viper.SetDefault("DB_NAME", "constructflow")
	viper.SetDefault("DB_USER", "admin")
	viper.SetDefault("DB_PASSWORD", "secret")
	viper.SetDefault("GRPC_PORT", 50053)
	viper.SetDefault("JWT_PRIVATE_KEY_PATH", "./keys/private.pem")
	viper.SetDefault("JWT_PUBLIC_KEY_PATH", "./keys/public.pem")

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

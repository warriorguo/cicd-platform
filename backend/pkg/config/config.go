package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	K8s      K8sConfig      `mapstructure:"k8s"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port string `mapstructure:"port"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

type K8sConfig struct {
	InCluster       bool   `mapstructure:"in_cluster"`
	KubeConfig      string `mapstructure:"kubeconfig"`
	PipelineNS      string `mapstructure:"pipeline_namespace"`
	TektonNamespace string `mapstructure:"tekton_namespace"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/cicd/")

	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("database.user", "postgres")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.dbname", "appdb")
	viper.SetDefault("database.password", "pass")
	viper.SetDefault("database.port", "5432")
	viper.SetDefault("database.sslmode", "disable")
	viper.SetDefault("k8s.in_cluster", false)
	viper.SetDefault("k8s.pipeline_namespace", "cicd-runners")
	viper.SetDefault("k8s.tekton_namespace", "tekton-pipelines")

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	K8s      K8sConfig      `mapstructure:"k8s"`
	Harbor   HarborConfig   `mapstructure:"harbor"`
	Defaults DefaultsConfig `mapstructure:"defaults"`
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

type HarborConfig struct {
	Endpoint string `mapstructure:"endpoint"`
	Project  string `mapstructure:"project"`
}

type DefaultsConfig struct {
	TargetNamespace   string `mapstructure:"target_namespace"`
	ContextPath       string `mapstructure:"context_path"`
	GitSecretRef      string `mapstructure:"git_secret_ref"`
	RegistrySecretRef string `mapstructure:"registry_secret_ref"`
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
	viper.SetDefault("harbor.endpoint", "harbor.example.com")
	viper.SetDefault("harbor.project", "library")
	viper.SetDefault("defaults.target_namespace", "default")
	viper.SetDefault("defaults.context_path", ".")
	viper.SetDefault("defaults.git_secret_ref", "")
	viper.SetDefault("defaults.registry_secret_ref", "")

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

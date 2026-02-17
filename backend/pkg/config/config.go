package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	K8s      K8sConfig      `mapstructure:"k8s"`
	Harbor   HarborConfig   `mapstructure:"harbor"`
	Defaults DefaultsConfig `mapstructure:"defaults"`
	Ingress  IngressConfig  `mapstructure:"ingress"`
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
	Enabled  bool   `mapstructure:"enabled"`
	Endpoint string `mapstructure:"endpoint"`
	URL      string `mapstructure:"url"`
	Project  string `mapstructure:"project"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type DefaultsConfig struct {
	TargetNamespace   string `mapstructure:"target_namespace"`
	ContextPath       string `mapstructure:"context_path"`
	GitSecretRef      string `mapstructure:"git_secret_ref"`
	RegistrySecretRef string `mapstructure:"registry_secret_ref"`
}

type IngressConfig struct {
	HostTemplate string `mapstructure:"host_template"`
	DefaultPath  string `mapstructure:"default_path"`
	TLSSecret    string `mapstructure:"tls_secret"`
}

func Load() (*Config, error) {
	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("..")  // Parent directory for when running from backend/
	v.AddConfigPath("/etc/cicd/")

	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", "8080")
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.dbname", "appdb")
	v.SetDefault("database.password", "pass")
	v.SetDefault("database.port", "5432")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("k8s.in_cluster", false)
	v.SetDefault("k8s.pipeline_namespace", "cicd-runners")
	v.SetDefault("k8s.tekton_namespace", "tekton-pipelines")
	v.SetDefault("harbor.enabled", false)
	v.SetDefault("harbor.endpoint", "harbor.example.com")
	v.SetDefault("harbor.url", "harbor.example.com")
	v.SetDefault("harbor.project", "library")
	v.SetDefault("harbor.username", "admin")
	v.SetDefault("harbor.password", "")
	v.SetDefault("defaults.target_namespace", "default")
	v.SetDefault("defaults.context_path", ".")
	v.SetDefault("defaults.git_secret_ref", "")
	v.SetDefault("defaults.registry_secret_ref", "")
	v.SetDefault("ingress.host_template", "*.localhost")
	v.SetDefault("ingress.default_path", "/")
	v.SetDefault("ingress.tls_secret", "")

	// Enable environment variable overrides with underscore separator
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

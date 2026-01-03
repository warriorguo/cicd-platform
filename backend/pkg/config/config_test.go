package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigDefaults(t *testing.T) {
	// Clear environment variables that might interfere
	os.Clearenv()

	config, err := Load()
	require.NoError(t, err)

	// Test default values
	assert.Equal(t, "0.0.0.0", config.Server.Host)
	assert.Equal(t, "8080", config.Server.Port)
	assert.Equal(t, "localhost", config.Database.Host)
	assert.Equal(t, "5432", config.Database.Port)
	assert.Equal(t, "disable", config.Database.SSLMode)
	assert.False(t, config.K8s.InCluster)
	assert.Equal(t, "cicd-runners", config.K8s.PipelineNS)
	assert.Equal(t, "tekton-pipelines", config.K8s.TektonNamespace)
}

func TestConfigEnvironmentVariables(t *testing.T) {
	// Set environment variables
	os.Setenv("SERVER_HOST", "127.0.0.1")
	os.Setenv("SERVER_PORT", "9090")
	os.Setenv("DATABASE_HOST", "postgres.example.com")
	os.Setenv("DATABASE_PORT", "5433")
	os.Setenv("DATABASE_USER", "testuser")
	os.Setenv("DATABASE_PASSWORD", "testpass")
	os.Setenv("DATABASE_DBNAME", "testdb")
	os.Setenv("DATABASE_SSLMODE", "require")
	os.Setenv("K8S_IN_CLUSTER", "true")
	os.Setenv("K8S_PIPELINE_NAMESPACE", "custom-runners")
	os.Setenv("K8S_TEKTON_NAMESPACE", "custom-tekton")
	os.Setenv("REGISTRY_DEFAULT_REGISTRY", "harbor.example.com")
	os.Setenv("REGISTRY_SECRET_NAME", "custom-secret")

	defer func() {
		// Clean up environment variables
		os.Clearenv()
	}()

	config, err := Load()
	require.NoError(t, err)

	// Test environment variable values
	assert.Equal(t, "127.0.0.1", config.Server.Host)
	assert.Equal(t, "9090", config.Server.Port)
	assert.Equal(t, "postgres.example.com", config.Database.Host)
	assert.Equal(t, "5433", config.Database.Port)
	assert.Equal(t, "testuser", config.Database.User)
	assert.Equal(t, "testpass", config.Database.Password)
	assert.Equal(t, "testdb", config.Database.DBName)
	assert.Equal(t, "require", config.Database.SSLMode)
	assert.True(t, config.K8s.InCluster)
	assert.Equal(t, "custom-runners", config.K8s.PipelineNS)
	assert.Equal(t, "custom-tekton", config.K8s.TektonNamespace)
}

func TestConfigStructure(t *testing.T) {
	config := &Config{
		Server: ServerConfig{
			Host: "test-host",
			Port: "test-port",
		},
		Database: DatabaseConfig{
			Host:     "db-host",
			Port:     "db-port",
			User:     "db-user",
			Password: "db-pass",
			DBName:   "db-name",
			SSLMode:  "db-ssl",
		},
		K8s: K8sConfig{
			InCluster:       true,
			KubeConfig:      "/path/to/kubeconfig",
			PipelineNS:      "pipeline-ns",
			TektonNamespace: "tekton-ns",
		},
	}

	assert.Equal(t, "test-host", config.Server.Host)
	assert.Equal(t, "test-port", config.Server.Port)
	assert.Equal(t, "db-host", config.Database.Host)
	assert.Equal(t, "db-port", config.Database.Port)
	assert.Equal(t, "db-user", config.Database.User)
	assert.Equal(t, "db-pass", config.Database.Password)
	assert.Equal(t, "db-name", config.Database.DBName)
	assert.Equal(t, "db-ssl", config.Database.SSLMode)
	assert.True(t, config.K8s.InCluster)
	assert.Equal(t, "/path/to/kubeconfig", config.K8s.KubeConfig)
	assert.Equal(t, "pipeline-ns", config.K8s.PipelineNS)
	assert.Equal(t, "tekton-ns", config.K8s.TektonNamespace)
}

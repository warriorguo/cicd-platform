package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
	"gorm.io/gorm"
)

type BuildType string

const (
	BuildTypeDockerfile     BuildType = "dockerfile"
	BuildTypeDockerCompose  BuildType = "docker-compose"
)

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type EnvVars []EnvVar

// Scan implements sql.Scanner interface
func (e *EnvVars) Scan(value interface{}) error {
	if value == nil {
		*e = []EnvVar{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, e)
}

// Value implements driver.Valuer interface
func (e EnvVars) Value() (driver.Value, error) {
	if len(e) == 0 {
		return nil, nil
	}
	return json.Marshal(e)
}

type App struct {
	ID               uint      `json:"id" gorm:"primaryKey"`
	Name             string    `json:"name" gorm:"uniqueIndex;not null"`
	GitURL           string    `json:"git_url" gorm:"not null"`
	DefaultBranch    string    `json:"default_branch" gorm:"default:'main'"`
	BuildType        BuildType `json:"build_type" gorm:"default:'dockerfile'"`
	DockerfilePath   string    `json:"dockerfile_path" gorm:"default:'Dockerfile'"`
	ContextPath      string    `json:"context_path" gorm:"default:'.'"`
	RegistryRepo     string    `json:"registry_repo" gorm:"not null"`
	TargetNamespace  string    `json:"target_namespace" gorm:"not null"`
	TargetDeployName string    `json:"target_deploy_name" gorm:"not null"`
	Replicas         int       `json:"replicas" gorm:"default:1"`
	GitSecretRef     string    `json:"git_secret_ref"`
	RegistrySecretRef string   `json:"registry_secret_ref"`
	EnvVars          EnvVars   `json:"env_vars" gorm:"type:jsonb"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`

	Releases []Release `json:"releases,omitempty" gorm:"foreignKey:AppID"`
}

type Release struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	AppID       uint           `json:"app_id" gorm:"not null;index"`
	Branch      string         `json:"branch" gorm:"not null"`
	CommitSHA   string         `json:"commit_sha"`
	ImageTag    string         `json:"image_tag"`
	ImageDigest string         `json:"image_digest"`
	Status      ReleaseStatus  `json:"status" gorm:"default:'pending'"`
	StartedAt   *time.Time     `json:"started_at"`
	FinishedAt  *time.Time     `json:"finished_at"`
	K8sRef      string         `json:"k8s_ref"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`

	App App `json:"app,omitempty" gorm:"foreignKey:AppID"`
}

type ReleaseStatus string

const (
	StatusPending  ReleaseStatus = "pending"
	StatusRunning  ReleaseStatus = "running"
	StatusSuccess  ReleaseStatus = "success"
	StatusFailed   ReleaseStatus = "failed"
	StatusCanceled ReleaseStatus = "canceled"
)

type Branch struct {
	Name string `json:"name"`
	SHA  string `json:"sha"`
}

type DockerfileValidation struct {
	Valid bool   `json:"valid"`
	Path  string `json:"path"`
	Error string `json:"error,omitempty"`
}
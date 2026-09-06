package config

import (
	"fmt"
	"os"
	"path"

	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	DebugMode             bool        `yaml:"debugMode"`
	Db                    *DbConfig   `yaml:"db"`
	SignKey               string      `yaml:"signKey"`
	DeviceEnrollmentKey   string      `yaml:"deviceEnrollmentKey"`
	ProvisioningSignSeed  string      `yaml:"provisioningSignSeed"`
	ProvisioningSecretKey string      `yaml:"provisioningSecretKey"`
	ProvisioningKeyId     string      `yaml:"provisioningKeyId"`
	HttpConfig            *HttpConfig `yaml:"httpConfig"`
	SmtpConfig            *SmtpConfig `yaml:"smtpConfig"`
	JobsConfig            *JobsConfig `yaml:"jobsConfig"`
}

type DbConfig struct {
	Driver   string `yaml:"driver"`
	Dsn      string `yaml:"dsn"`
	TimeZone string `yaml:"timeZone"`
	ShowSql  bool   `yaml:"showSql"`
}

type HttpConfig struct {
	PrintRequestLog bool   `yaml:"printRequestLog"`
	Port            string `yaml:"port"`
	StaticDir       string `yaml:"staticdir"`
}

type SmtpConfig struct {
	Host       string `yaml:"host"`
	Port       int    `yaml:"port"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
	Encryption string `yaml:"encryption"` // none ssl/tls starttls
	From       string `yaml:"from"`
}

type DeviceCheckJob struct {
	Duration int `yaml:"duration"`
}

type JobsConfig struct {
	DeviceCheckJob *DeviceCheckJob `yaml:"deviceCheckJob"`
}

var (
	wd, _    = os.Getwd()
	yamlFile = path.Join(wd, "server.yaml")
)

func GetDefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		DebugMode: false,
		Db: &DbConfig{
			Driver:   "sqlite",
			Dsn:      "./server.db",
			ShowSql:  true,
			TimeZone: "Asia/Shanghai",
		},
		HttpConfig: &HttpConfig{
			Port:      ":8080",
			StaticDir: "dist",
		},
		JobsConfig: &JobsConfig{
			DeviceCheckJob: &DeviceCheckJob{
				Duration: 30,
			},
		},
	}
}

func GetServerConfig() *ServerConfig {
	cfg := GetDefaultServerConfig()
	bytes, err := os.ReadFile(yamlFile)
	if err != nil {
		WriteServerConfig(cfg)
		applyEnvironment(cfg)
		return cfg
	}

	err = yaml.Unmarshal(bytes, cfg)
	if err != nil {
		WriteServerConfig(cfg)
		applyEnvironment(cfg)
		return cfg
	}
	applyEnvironment(cfg)
	return cfg
}

func applyEnvironment(cfg *ServerConfig) {
	if value := os.Getenv("RUD_API_SIGN_KEY"); value != "" {
		cfg.SignKey = value
	}
	if enrollmentKey := os.Getenv("RUD_DEVICE_ENROLLMENT_KEY_B64"); enrollmentKey != "" {
		cfg.DeviceEnrollmentKey = enrollmentKey
	}
	if value := os.Getenv("RUD_CFG_SIGN_SEED_B64"); value != "" {
		cfg.ProvisioningSignSeed = value
	}
	if value := os.Getenv("RUD_CFG_SECRETBOX_KEY_B64"); value != "" {
		cfg.ProvisioningSecretKey = value
	}
	if value := os.Getenv("RUD_CFG_KEY_ID"); value != "" {
		cfg.ProvisioningKeyId = value
	}
	if cfg.ProvisioningKeyId == "" {
		cfg.ProvisioningKeyId = "android-v1"
	}
}

func (cfg *ServerConfig) ValidateForServer() error {
	if len(cfg.SignKey) < 32 {
		return fmt.Errorf("RUD_API_SIGN_KEY or signKey must contain at least 32 characters")
	}
	return nil
}

func WriteServerConfig(cfg *ServerConfig) {
	bytes, _ := yaml.Marshal(cfg)
	_ = os.WriteFile(yamlFile, bytes, 0755)
}

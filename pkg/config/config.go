package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	HTTP       HTTPConfig       `envconfig:"HTTP"`
	OpenSearch OpenSearchConfig `envconfig:"OPENSEARCH"`
	QUS        QUSConfig        `envconfig:"QUS"`
	ConfigDir  string           `envconfig:"CONFIG_DIR" default:"configs"`
}

type HTTPConfig struct {
	Port int `envconfig:"PORT" default:"8081"`
}

type OpenSearchConfig struct {
	URL      string        `envconfig:"URL" default:"http://localhost:9200"`
	Username string        `envconfig:"USERNAME"`
	Password string        `envconfig:"PASSWORD"`
	Timeout  time.Duration `envconfig:"TIMEOUT" default:"5s"`
}

type QUSConfig struct {
	URL     string        `envconfig:"URL" default:"http://localhost:8080"`
	Timeout time.Duration `envconfig:"TIMEOUT" default:"3s"`
}

func Load(prefix string) (Config, error) {
	var cfg Config
	if err := envconfig.Process(prefix, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.HTTP.Port <= 0 || c.HTTP.Port > 65535 {
		return fmt.Errorf("invalid HTTP port: %d", c.HTTP.Port)
	}
	if c.OpenSearch.URL == "" {
		return fmt.Errorf("OpenSearch URL is required")
	}
	if c.QUS.URL == "" {
		return fmt.Errorf("QUS URL is required")
	}
	if c.ConfigDir == "" {
		return fmt.Errorf("config directory is required")
	}
	return nil
}

package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type SearchConfig struct {
	Index          string            `yaml:"index"`
	Stages         []StageConfig     `yaml:"stages"`
	DefaultFilters []FilterConfig    `yaml:"default_filters"`
	Sorts          map[string][]Sort `yaml:"sorts"`
	Facets         []FacetConfig     `yaml:"facets"`
}

type StageConfig struct {
	Name           string        `yaml:"name"`
	MinimumHits    int           `yaml:"minimum_hits"`
	QueryMode      string        `yaml:"query_mode"` // "exact" or "partial"
	OmitPercentage int           `yaml:"omit_percentage,omitempty"`
	MaxTermCount   int           `yaml:"max_term_count,omitempty"`
	Fields         []FieldConfig `yaml:"fields"`
}

type FieldConfig struct {
	Name  string  `yaml:"name"`
	Boost float64 `yaml:"boost"`
}

type FilterConfig struct {
	Field    string `yaml:"field"`
	Operator string `yaml:"operator"`
	Value    any    `yaml:"value"`
}

type Sort struct {
	Field     string `yaml:"field"`
	Direction string `yaml:"direction"`
}

type FacetConfig struct {
	Field       string `yaml:"field"`
	Type        string `yaml:"type"` // "terms" or "range"
	Size        int    `yaml:"size"`
	ExcludeSelf bool   `yaml:"exclude_self"`
}

func LoadSearchConfig(path string) (SearchConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SearchConfig{}, fmt.Errorf("reading %s: %w", path, err)
	}

	var cfg SearchConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return SearchConfig{}, fmt.Errorf("parsing %s: %w", path, err)
	}

	return cfg, nil
}

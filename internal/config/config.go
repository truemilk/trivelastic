package config

import (
	"fmt"
	"os"

	"github.com/truemilk/trivelastic/internal/logger"
)

type Config struct {
	Port string
	ES   ElasticsearchConfig
	Log  LogConfig
}

type ElasticsearchConfig struct {
	URL    string
	APIKey string
	Index  string
}

type LogConfig struct {
	Level      string
	JSONFormat bool
}

func Load() (*Config, error) {
	// Initialize logger with basic configuration for config loading
	err := logger.Initialize(logger.Config{
		Level:      os.Getenv("LOG_LEVEL"),
		JSONFormat: os.Getenv("LOG_FORMAT") == "json",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	log := logger.GetLogger("config")

	// Load Elasticsearch config
	esConfig, err := loadESConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load Elasticsearch configuration")
		return nil, err
	}

	// Load server port
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Info().Str("port", port).Msg("Using default port")
	} else {
		log.Info().Str("port", port).Msg("Port configured from environment")
	}

	// Load logging config
	logConfig := loadLogConfig()
	log.Info().
		Str("level", logConfig.Level).
		Bool("json_format", logConfig.JSONFormat).
		Msg("Logging configuration loaded")

	config := &Config{
		Port: port,
		ES:   *esConfig,
		Log:  *logConfig,
	}

	log.Info().Msg("Configuration loaded successfully")
	return config, nil
}

func loadESConfig() (*ElasticsearchConfig, error) {
	log := logger.GetLogger("config.elasticsearch")

	url := os.Getenv("ES_URL")
	apiKey := os.Getenv("ES_API_KEY")
	index := os.Getenv("ES_INDEX")

	missingVars := make([]string, 0)
	if url == "" {
		missingVars = append(missingVars, "ES_URL")
	}
	if apiKey == "" {
		missingVars = append(missingVars, "ES_API_KEY")
	}
	if index == "" {
		missingVars = append(missingVars, "ES_INDEX")
	}

	if len(missingVars) > 0 {
		log.Error().
			Strs("missing_variables", missingVars).
			Msg("Missing required environment variables")
		return nil, fmt.Errorf("missing required environment variables: %v", missingVars)
	}

	config := &ElasticsearchConfig{
		URL:    url,
		APIKey: apiKey,
		Index:  index,
	}

	log.Info().
		Str("url", url).
		Str("index", index).
		Msg("Elasticsearch configuration loaded")

	return config, nil
}

func loadLogConfig() *LogConfig {
	log := logger.GetLogger("config.log")

	level := os.Getenv("LOG_LEVEL")
	if level == "" {
		level = "info"
		log.Info().Str("level", level).Msg("Using default log level")
	}

	jsonFormat := os.Getenv("LOG_FORMAT") == "json"
	format := "console"
	if jsonFormat {
		format = "json"
	}

	log.Debug().
		Str("level", level).
		Str("format", format).
		Msg("Log configuration loaded")

	return &LogConfig{
		Level:      level,
		JSONFormat: jsonFormat,
	}
}

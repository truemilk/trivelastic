package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Config holds logger configuration
type Config struct {
	Level      string
	JSONFormat bool
}

// Initialize sets up the global logger with the given configuration
func Initialize(cfg Config) error {
	// Set logger time format
	zerolog.TimeFieldFormat = time.RFC3339

	// Set global logger
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel // Default to info level on error
	}
	zerolog.SetGlobalLevel(level)

	// Configure output writer
	var output io.Writer = os.Stdout

	// Configure console writer for local development
	if !cfg.JSONFormat {
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
			NoColor:    true, // Disable colors for better compatibility
		}
	}

	// Set up the global logger
	log.Logger = zerolog.New(output).With().Timestamp().Logger()

	return nil
}

// GetLogger returns a logger instance with the given component name
func GetLogger(component string) zerolog.Logger {
	return log.With().Str("component", component).Logger()
}

// Debug logs a debug message
func Debug(msg string, fields ...interface{}) {
	log.Debug().Fields(fieldsToMap(fields...)).Msg(msg)
}

// Info logs an info message
func Info(msg string, fields ...interface{}) {
	log.Info().Fields(fieldsToMap(fields...)).Msg(msg)
}

// Warn logs a warning message
func Warn(msg string, fields ...interface{}) {
	log.Warn().Fields(fieldsToMap(fields...)).Msg(msg)
}

// Error logs an error message
func Error(msg string, err error, fields ...interface{}) {
	logEvent := log.Error().Fields(fieldsToMap(fields...))
	if err != nil {
		logEvent = logEvent.Err(err)
	}
	logEvent.Msg(msg)
}

// Fatal logs a fatal message and exits
func Fatal(msg string, err error, fields ...interface{}) {
	logEvent := log.Fatal().Fields(fieldsToMap(fields...))
	if err != nil {
		logEvent = logEvent.Err(err)
	}
	logEvent.Msg(msg)
}

// fieldsToMap converts a slice of interfaces to a map for structured logging
func fieldsToMap(fields ...interface{}) map[string]interface{} {
	if len(fields)%2 != 0 {
		log.Warn().Msg("Fields must be provided in pairs")
		return nil
	}

	result := make(map[string]interface{}, len(fields)/2)
	for i := 0; i < len(fields); i += 2 {
		key, ok := fields[i].(string)
		if !ok {
			log.Warn().Msgf("Field key must be string, got %T", fields[i])
			continue
		}
		result[key] = fields[i+1]
	}
	return result
}

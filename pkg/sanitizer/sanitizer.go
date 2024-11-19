package sanitizer

import (
	"fmt"

	"github.com/truemilk/trivelastic/internal/logger"
)

// SanitizeJSON cleans up the JSON data by removing empty values and sanitizing special fields
func SanitizeJSON(data map[string]interface{}) map[string]interface{} {
	log := logger.GetLogger("sanitizer")
	log.Debug().Interface("input", data).Msg("Starting JSON sanitization")

	result := make(map[string]interface{})
	for key, value := range data {
		// Skip fields that are just dots
		if key == "." || key == ".." {
			log.Debug().Str("key", key).Msg("Skipping dot field")
			continue
		}

		// Handle empty date fields
		if key == "lastModifiedDate" && (value == "" || value == nil) {
			log.Debug().Str("key", key).Msg("Skipping empty date field")
			continue
		}

		// Recursively handle nested objects
		switch v := value.(type) {
		case map[string]interface{}:
			log.Debug().Str("key", key).Msg("Processing nested object")
			sanitized := SanitizeJSON(v)
			if len(sanitized) > 0 { // Only add non-empty objects
				result[key] = sanitized
			} else {
				log.Debug().Str("key", key).Msg("Skipping empty nested object")
			}
		case []interface{}:
			log.Debug().Str("key", key).Msg("Processing array")
			sanitized := sanitizeArray(v)
			if len(sanitized) > 0 { // Only add non-empty arrays
				result[key] = sanitized
			} else {
				log.Debug().Str("key", key).Msg("Skipping empty array")
			}
		case string:
			if v != "" { // Only add non-empty strings
				result[key] = value
				log.Debug().Str("key", key).Msg("Added non-empty string")
			} else {
				log.Debug().Str("key", key).Msg("Skipping empty string")
			}
		default:
			if value != nil { // Only add non-nil values
				result[key] = value
				log.Debug().
					Str("key", key).
					Str("type", fmt.Sprintf("%T", value)).
					Msg("Added non-nil value")
			} else {
				log.Debug().Str("key", key).Msg("Skipping nil value")
			}
		}
	}

	log.Debug().
		Int("input_size", len(data)).
		Int("output_size", len(result)).
		Msg("JSON sanitization completed")

	return result
}

// sanitizeArray handles array values in the JSON
func sanitizeArray(arr []interface{}) []interface{} {
	log := logger.GetLogger("sanitizer.array")
	log.Debug().Int("array_length", len(arr)).Msg("Starting array sanitization")

	result := make([]interface{}, 0, len(arr))
	for i, value := range arr {
		switch v := value.(type) {
		case map[string]interface{}:
			log.Debug().Int("index", i).Msg("Processing object in array")
			sanitized := SanitizeJSON(v)
			if len(sanitized) > 0 {
				result = append(result, sanitized)
			} else {
				log.Debug().Int("index", i).Msg("Skipping empty object in array")
			}
		case []interface{}:
			log.Debug().Int("index", i).Msg("Processing nested array")
			sanitized := sanitizeArray(v)
			if len(sanitized) > 0 {
				result = append(result, sanitized)
			} else {
				log.Debug().Int("index", i).Msg("Skipping empty nested array")
			}
		default:
			if value != nil {
				result = append(result, value)
				log.Debug().
					Int("index", i).
					Str("type", fmt.Sprintf("%T", value)).
					Msg("Added non-nil value to array")
			} else {
				log.Debug().Int("index", i).Msg("Skipping nil value in array")
			}
		}
	}

	log.Debug().
		Int("input_length", len(arr)).
		Int("output_length", len(result)).
		Msg("Array sanitization completed")

	return result
}

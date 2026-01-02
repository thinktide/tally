package config

import (
	"github.com/thinktide/tally/internal/db"
)

// KeyOutputFormat is the configuration key for specifying the format of the output.
// KeyDataLocation is the configuration key for specifying the location of the data.
const (
	KeyOutputFormat = "output.format"
	KeyDataLocation = "data.location"
)

// defaults is a map defining the default configuration values for specific keys used in the application.
//
// It contains predefined key-value pairs that act as fallback values when no explicit configuration is provided.
// The keys are constants like [KeyOutputFormat] and [KeyDataLocation].
//
// This map is consulted whenever retrieving or validating configuration data, ensuring consistent behavior across the application.
var defaults = map[string]string{
	KeyOutputFormat: "table",
	KeyDataLocation: "~/.tally",
}

// Get retrieves the configuration value associated with the given key.
//
// If the key exists in the database, its value is returned. If the key does not exist in the database
// but has a default value defined in the [defaults] map, the default value is returned instead.
//
// - key: The configuration key to look up.
//
// Returns the value as a string if found. Returns an empty string and nil error if the key has no value
// in the database but a default value exists. Returns an error if fetching from the database fails or
// the key is entirely unknown.
func Get(key string) (string, error) {
	value, err := db.GetConfig(key)
	if err != nil {
		return "", err
	}
	if value == "" {
		if def, ok := defaults[key]; ok {
			return def, nil
		}
	}
	return value, nil
}

// Set updates the configuration by saving the provided key-value pair persistently.
//
// The function stores the key-value pair in the application's configuration storage. If the key already exists,
// its value will be replaced. Invalid key or value handling is expected to be done prior to calling this function.
//
// Errors may occur under the following circumstances:
//   - If there is an issue with the underlying storage operation.
//   - If the database connection [DB] is unavailable.
//
// Returns an error if the operation fails.
func Set(key, value string) error {
	return db.SetConfig(key, value)
}

// List retrieves a combined map of configuration settings by merging stored and default values.
//
// It first fetches existing configurations using [db.ListConfig]. Default values from [defaults] are then layered on top,
// with actual stored values taking precedence to ensure all configuration keys are represented.
//
// Returns a map of the merged configurations with `stored` values overriding `defaults`. In case of an error during the
// retrieval of stored configurations, it returns a non-nil error.
func List() (map[string]string, error) {
	stored, err := db.ListConfig()
	if err != nil {
		return nil, err
	}

	// Merge with defaults
	result := make(map[string]string)
	for k, v := range defaults {
		result[k] = v
	}
	for k, v := range stored {
		result[k] = v
	}
	return result, nil
}

// GetBool retrieves the boolean value associated with the provided key.
//
// It fetches the string value for the given key using [Get] and interprets it as a boolean.
// The string "true" (case-sensitive) will be converted to true. All other values, including an empty string,
// default to false.
//
// If an error occurs during retrieval, the function returns false along with the error.
//
// Returns:
//   - The interpreted boolean value for the key.
//   - An error if the key does not exist or if any issue occurs during retrieval.
func GetBool(key string) (bool, error) {
	value, err := Get(key)
	if err != nil {
		return false, err
	}
	return value == "true", nil
}

// SetBool sets a boolean value in the configuration for the given key.
//
// The parameter key specifies the configuration key to update, and value determines the boolean value to set.
// The underlying implementation converts the boolean value to its string equivalent ("true" or "false").
//
// Returns an error if the operation fails due to database interaction issues.
func SetBool(key string, value bool) error {
	v := "false"
	if value {
		v = "true"
	}
	return Set(key, v)
}

// ValidKeys returns a list of all valid configuration keys.
//
// The keys are derived from the default configuration values stored in an internal map.
// This allows dynamic retrieval of supported configuration keys with minimal maintenance.
//
// The returned slice is dynamically constructed, ensuring it reflects any updates to the default keys.
func ValidKeys() []string {
	keys := make([]string, 0, len(defaults))
	for k := range defaults {
		keys = append(keys, k)
	}
	return keys
}

// IsValidKey checks whether the provided key exists in the default configuration map.
//
// This function verifies that the given key is defined in the internal `defaults` map.
//
// - key: A string representing the configuration key to verify.
//
// Returns true if the key exists in the defaults map, otherwise false.
func IsValidKey(key string) bool {
	_, ok := defaults[key]
	return ok
}

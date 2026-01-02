package config

import (
	"github.com/thinktide/tally/internal/db"
)

const (
	KeyOutputFormat = "output.format"
	KeyDataLocation = "data.location"
)

var defaults = map[string]string{
	KeyOutputFormat: "table",
	KeyDataLocation: "~/.tally",
}

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

func Set(key, value string) error {
	return db.SetConfig(key, value)
}

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

func GetBool(key string) (bool, error) {
	value, err := Get(key)
	if err != nil {
		return false, err
	}
	return value == "true", nil
}

func SetBool(key string, value bool) error {
	v := "false"
	if value {
		v = "true"
	}
	return Set(key, v)
}

func ValidKeys() []string {
	keys := make([]string, 0, len(defaults))
	for k := range defaults {
		keys = append(keys, k)
	}
	return keys
}

func IsValidKey(key string) bool {
	_, ok := defaults[key]
	return ok
}

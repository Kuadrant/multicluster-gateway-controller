package env

import (
	"os"
	"strconv"
)

func GetEnvString(key, fallback string) string {
	value, found := os.LookupEnv(key)
	if !found {
		return fallback
	}
	return value
}

func GetEnvBool(key string, fallback bool) bool {
	strValue, found := os.LookupEnv(key)
	if !found {
		return fallback
	}
	value, err := strconv.ParseBool(strValue)
	if err != nil {
		return fallback
	}
	return value
}

func GetEnvInt(key string, fallback int) int {
	strValue, found := os.LookupEnv(key)
	if !found {
		return fallback
	}
	value, err := strconv.Atoi(strValue)
	if err != nil {
		return fallback
	}
	return value
}

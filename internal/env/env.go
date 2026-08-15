package env

import (
	"log"
	"os"
	"strconv"
)

func GetEnvInt(key string, defaultValue int) int {
	envStr, ok := os.LookupEnv(key)
	if !ok {
		log.Printf("Environment variable %s not set, using default value: %d", key, defaultValue)
		return defaultValue
	}
	envInt, err := strconv.Atoi(envStr)
	if err != nil {
		log.Printf(
			"Error converting environment variable %s to int: %v, using default value: %d",
			key, err, defaultValue)
		return defaultValue
	}

	return envInt
}

func GetEnvString(key, defaultValue string) string {
	env, ok := os.LookupEnv(key)
	if !ok {
		log.Printf("Environment variable %s not set, using default value: %s", key, defaultValue)
		return defaultValue
	}

	return env
}

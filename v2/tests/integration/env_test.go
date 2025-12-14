package integration

import "os"

func lookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}

func setEnv(key, value string) {
	_ = os.Setenv(key, value) // Ignore error in test helper
}

func unsetEnv(key string) {
	_ = os.Unsetenv(key) // Ignore error in test helper
}


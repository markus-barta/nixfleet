package integration

import "os"

func lookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}

func setEnv(key, value string) {
	os.Setenv(key, value)
}

func unsetEnv(key string) {
	os.Unsetenv(key)
}


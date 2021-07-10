package env

import (
	"log"
	"os"
	"strconv"
)

var ParseBool = func(value *bool, envName string) {
	var err error
	// peerServiceAdvertisedPort
	env := os.Getenv(envName)
	if env == "" {
		log.Fatalf("need env.%s", env)
	}
	*value, err = strconv.ParseBool(env)
	if err != nil {
		log.Fatalf("invalid env.%s", envName)
	}
}

var ParseStr = func(value *string, envName string) {
	// peerServiceAdvertisedHost
	*value = os.Getenv(envName)
	if *value == "" {
		log.Fatalf("need env.%s", envName)
	}
}

var ParseIntEnv = func(value *int64, envName string) {
	var err error
	// peerServiceAdvertisedPort
	env := os.Getenv(envName)
	if env == "" {
		log.Fatalf("need env.%s", env)
	}
	*value, err = strconv.ParseInt(env, 10, 32)
	if err != nil {
		log.Fatalf("invalid env.%s", envName)
	}
}

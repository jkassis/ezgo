package env

import (
	"log"
	"os"
	"strconv"
)

// ParseBool parses a bool from env var
var ParseBool = func(value *bool, envName string) {
	var err error
	// peerServiceAdvertisedPort
	env := os.Getenv(envName)
	if env == "" {
		log.Fatalf("need env.%s", envName)
	}
	*value, err = strconv.ParseBool(env)
	if err != nil {
		log.Fatalf("invalid env.%s", envName)
	}
}

// ParseStr parses a str from env var
var ParseStr = func(value *string, envName string) {
	// peerServiceAdvertisedHost
	*value = os.Getenv(envName)
	if *value == "" {
		log.Fatalf("need env.%s", envName)
	}
}

// ParseInt parses a int from env var
var ParseInt = func(value *int64, envName string) {
	var err error
	// peerServiceAdvertisedPort
	env := os.Getenv(envName)
	if env == "" {
		log.Fatalf("need env.%s", envName)
	}
	*value, err = strconv.ParseInt(env, 10, 32)
	if err != nil {
		log.Fatalf("invalid env.%s", envName)
	}
}

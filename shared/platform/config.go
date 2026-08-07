package platform

import (
	"fmt"
	"os"
)

type Platform struct {
	IngressURL  string
	EgressURL   string
	DatabaseURL string
}

func Load() Platform {
	return Platform{
		IngressURL:  getenv("INGRESS_URL", "rtmp://localhost:5554"),
		EgressURL:   getenv("EGRESS_URL", "http://localhosst:5553"),
		DatabaseURL: getenv("DATABASE_URL", "postgres://root:root@127.0.0.1:5432/dev_finnio"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func MustGetenv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("%s is required", key))
	}
	return v
}

package config

import (
	"os"
	"strings"
)

type Config struct {
	Name       string
	Addr       string
	DataDir    string
	IngressURL string
	HLSAbrDir  string
	PublicURL  string
}

func New(ingressURL string) Config {
	if ingressURL == "" {
		ingressURL = getenv("INGRESS_URL", "rtmp://localhost:5554/live")
	}
	return Config{
		Name:       "API config",
		Addr:       getenv("API_addr", ":5555"),
		DataDir:    getenv("DATA_DIR", "data"),
		IngressURL: ingressURL,
		HLSAbrDir:  getenv("HLS_ABR_DIR", "tmp/hls/abr"),
		PublicURL:  strings.TrimRight(getenv("PUBLIC_URL", "http://localhost:5555"), "/"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

package traefik_github_oauth_server

import (
	"os"

	"github.com/spf13/cast"
)

type Config struct {
	ApiBaseURL              string
	ApiSecretKey            string
	ServerAddress           string
	DebugMode               bool
	GitHubOAuthClientID     string
	GitHubOAuthClientSecret string
}

func NewConfigFromEnv() *Config {
	return &Config{
		ApiBaseURL:              os.Getenv("API_BASE_URL"),
		ApiSecretKey:            os.Getenv("API_SECRET_KEY"),
		ServerAddress:           os.Getenv("SERVER_ADDRESS"),
		DebugMode:               cast.ToBool(os.Getenv("DEBUG_MODE")),
		GitHubOAuthClientID:     os.Getenv("GITHUB_OAUTH_CLIENT_ID"),
		GitHubOAuthClientSecret: os.Getenv("GITHUB_OAUTH_CLIENT_SECRET"),
	}
}

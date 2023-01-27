package traefik_github_oauth_server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/patrickmn/go-cache"
	"github.com/rs/zerolog"
	"golang.org/x/oauth2"
	oauth2github "golang.org/x/oauth2/github"
)

func init() {
	gin.SetMode(gin.ReleaseMode)
}

// App the Traefik GitHub OAuth server application.
type App struct {
	Config             *Config
	Server             *http.Server
	Engine             *gin.Engine
	GitHubOAuthConfig  *oauth2.Config
	AuthRequestManager *AuthRequestManager
	Logger             *zerolog.Logger
}

func NewApp(
	config *Config,
	server *http.Server,
	engine *gin.Engine,
	authRequestManager *AuthRequestManager,
	logger *zerolog.Logger,
) *App {
	gin.DebugPrintRouteFunc = ginDebugPrintRouteFunc(logger)
	if config.DebugMode {
		gin.SetMode(gin.DebugMode)
		config.LogLevel = "DEBUG"
	}

	switch config.LogLevel {
	case "DEBUG", "debug":
		logger.Level(zerolog.DebugLevel)
	case "INFO", "info":
		logger.Level(zerolog.InfoLevel)
	case "WARNING", "warning", "WARN", "warn":
		logger.Level(zerolog.WarnLevel)
	case "ERROR", "error":
		logger.Level(zerolog.ErrorLevel)
	}

	server.Addr = config.ServerAddress
	server.Handler = engine

	app := &App{
		Config: config,
		Server: server,
		Engine: engine,
		GitHubOAuthConfig: &oauth2.Config{
			ClientID:     config.GitHubOAuthClientID,
			ClientSecret: config.GitHubOAuthClientSecret,
			Endpoint:     oauth2github.Endpoint,
		},
		AuthRequestManager: authRequestManager,
		Logger:             logger,
	}

	return app
}

func NewDefaultApp() *App {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	engine := gin.New()
	engine.Use(NewLoggerMiddleware(&logger), gin.Recovery())
	return NewApp(
		NewConfigFromEnv(),
		&http.Server{
			ReadHeaderTimeout: 5 * time.Second,
		},
		engine,
		NewAuthRequestManager(cache.New(10*time.Minute, 30*time.Minute)),
		&logger,
	)
}

func (app *App) Run() {
	go func() {
		if err := app.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			app.Logger.Fatal().Err(err).Msgf("Failed to listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	app.Logger.Info().Msg("Shutdown Server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := app.Server.Shutdown(ctx); err != nil {
		app.Logger.Fatal().Err(err).Msgf("Error while shutting down server: %s\n", err)
	}
	defer cancel()
	app.Logger.Info().Msg("Server exiting")
}

func ginDebugPrintRouteFunc(logger *zerolog.Logger) func(httpMethod, absolutePath, handlerName string, nuHandlers int) {
	return func(httpMethod, absolutePath, handlerName string, nuHandlers int) {
		logger.Debug().
			Str("module", "gin-debug").
			Str("http_method", httpMethod).
			Str("path", absolutePath).
			Str("handler_name", handlerName).
			Int("num_handlers", nuHandlers).
			Msg("")
	}
}

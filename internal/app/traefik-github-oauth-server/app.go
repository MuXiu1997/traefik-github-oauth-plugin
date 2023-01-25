package traefik_github_oauth_server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/patrickmn/go-cache"
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
	AuthRequestManager *cache.Cache
}

func NewApp(config *Config, server *http.Server, engine *gin.Engine, authRequestManager *cache.Cache) *App {
	server.Addr = config.ServerAddress
	server.Handler = engine

	if config.DebugMode {
		gin.SetMode(gin.DebugMode)
	}

	app := &App{
		Config:             config,
		Server:             server,
		Engine:             engine,
		AuthRequestManager: authRequestManager,

		GitHubOAuthConfig: &oauth2.Config{
			ClientID:     config.GitHubOAuthClientID,
			ClientSecret: config.GitHubOAuthClientSecret,
			Endpoint:     oauth2github.Endpoint,
		},
	}

	return app
}

func NewDefaultApp() *App {
	return NewApp(
		NewConfigFromEnv(),
		&http.Server{
			ReadHeaderTimeout: 5 * time.Second,
		},
		gin.Default(),
		cache.New(10*time.Minute, 30*time.Minute),
	)
}

func (app *App) Run() {
	go func() {
		if err := app.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Shutdown Server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := app.Server.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}
	defer cancel()
	log.Println("Server exiting")
}

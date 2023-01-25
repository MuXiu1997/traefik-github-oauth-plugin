package main

import (
	. "github.com/muxiu1997/traefik-github-oauth-plugin/internal/app/traefik-github-oauth-server"
	"github.com/muxiu1997/traefik-github-oauth-plugin/internal/app/traefik-github-oauth-server/router"
)

func main() {
	app := NewDefaultApp()
	router.RegisterRoutes(app)
	app.Run()
}

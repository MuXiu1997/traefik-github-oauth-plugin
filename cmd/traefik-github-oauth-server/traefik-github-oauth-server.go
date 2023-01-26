package main

import (
	. "github.com/MuXiu1997/traefik-github-oauth-plugin/internal/app/traefik-github-oauth-server"
	"github.com/MuXiu1997/traefik-github-oauth-plugin/internal/app/traefik-github-oauth-server/router"
)

func main() {
	app := NewDefaultApp()
	router.RegisterRoutes(app)
	app.Run()
}

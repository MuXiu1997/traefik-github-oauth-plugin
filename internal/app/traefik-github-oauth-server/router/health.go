package router

import (
	"net/http"

	server "github.com/MuXiu1997/traefik-github-oauth-plugin/internal/app/traefik-github-oauth-server"
	"github.com/gin-gonic/gin"
)

func healthCheck(_ *server.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Status(http.StatusOK)
	}
}

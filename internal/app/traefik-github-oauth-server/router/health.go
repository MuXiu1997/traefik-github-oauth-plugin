package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	server "github.com/muxiu1997/traefik-github-oauth-plugin/internal/app/traefik-github-oauth-server"
)

func healthCheck(_ *server.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Status(http.StatusOK)
	}
}

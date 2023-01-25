package traefik_github_oauth_server

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/muxiu1997/traefik-github-oauth-plugin/internal/pkg/constant"
)

func NewApiSecretKeyMiddleware(apiSecretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(apiSecretKey) == 0 {
			c.Next()
			return
		}
		reqSecretKey := c.GetHeader(constant.HTTP_HEADER_AUTHORIZATION)
		if reqSecretKey != fmt.Sprintf("%s %s", constant.AUTHORIZATION_PREFIX_TOKEN, apiSecretKey) {
			c.AbortWithStatus(http.StatusUnauthorized)
		}
	}
}

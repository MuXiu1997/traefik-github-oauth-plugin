package traefik_github_oauth_server

import (
	"fmt"
	"net/http"

	"github.com/MuXiu1997/traefik-github-oauth-plugin/internal/app/traefik-github-oauth-server/model"
	"github.com/MuXiu1997/traefik-github-oauth-plugin/internal/pkg/constant"
	"github.com/gin-gonic/gin"
)

// NewApiSecretKeyMiddleware returns a middleware that checks the api secret key.
func NewApiSecretKeyMiddleware(apiSecretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(apiSecretKey) == 0 {
			c.Next()
			return
		}
		reqSecretKey := c.GetHeader(constant.HTTP_HEADER_AUTHORIZATION)
		if reqSecretKey != fmt.Sprintf("%s %s", constant.AUTHORIZATION_PREFIX_TOKEN, apiSecretKey) {
			c.JSON(http.StatusUnauthorized, model.ResponseError{
				Message: "invalid api secret key",
			})
		}
		c.Next()
	}
}

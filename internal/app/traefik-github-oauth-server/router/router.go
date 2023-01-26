package router

import (
	"net/http"

	server "github.com/MuXiu1997/traefik-github-oauth-plugin/internal/app/traefik-github-oauth-server"
	"github.com/MuXiu1997/traefik-github-oauth-plugin/internal/pkg/constant"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(app *server.App) {
	apiSecretKeyMiddleware := server.NewApiSecretKeyMiddleware(app.Config.ApiSecretKey)

	app.Engine.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Traefik GitHub OAuth Server")
	})

	app.Engine.GET(constant.ROUTER_PATH_OAUTH_HEALTH, healthCheck(app))

	oauthGroup := app.Engine.Group(constant.ROUTER_GROUP_PATH_OAUTH)
	oauthGroup.POST(
		constant.ROUTER_PATH_OAUTH_PAGE_URL,
		apiSecretKeyMiddleware,
		generateOAuthPageURL(app),
	)
	oauthGroup.GET(constant.ROUTER_PATH_OAUTH_REDIRECT, redirect(app))
	oauthGroup.GET(
		constant.ROUTER_PATH_OAUTH_RESULT,
		apiSecretKeyMiddleware,
		getAuthResult(app),
	)
}

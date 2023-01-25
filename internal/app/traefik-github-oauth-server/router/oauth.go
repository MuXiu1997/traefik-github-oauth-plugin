package router

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/google/go-github/v49/github"
	server "github.com/muxiu1997/traefik-github-oauth-plugin/internal/app/traefik-github-oauth-server"
	"github.com/muxiu1997/traefik-github-oauth-plugin/internal/app/traefik-github-oauth-server/model"
	"github.com/muxiu1997/traefik-github-oauth-plugin/internal/pkg/constant"
	"github.com/rs/xid"
	"github.com/spf13/cast"
	"golang.org/x/oauth2"
)

func generateOAuthPageURL(app *server.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		body := model.RequestGenerateOAuthPageURL{}
		err := c.BindJSON(&body)
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}

		rid := xid.New().String()
		app.AuthRequestManager.SetDefault(rid, &model.AuthRequest{
			RedirectURI: body.RedirectURI,
			AuthURL:     body.AuthURL,
		})

		redirectURI, err := buildRedirectURI(app.Config.ApiBaseURL, rid)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		oAuthPageURL := app.GitHubOAuthConfig.AuthCodeURL(
			"",
			oauth2.SetAuthURLParam(constant.QUERY_KEY_REDIRECT_URI, redirectURI),
		)

		c.JSON(
			http.StatusCreated,
			model.ResponseGenerateOAuthPageURL{
				OAuthPageURL: oAuthPageURL,
			},
		)
	}
}

func redirect(app *server.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := model.RequestRedirect{}
		err := c.BindQuery(&query)
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		authRequestCache, found := app.AuthRequestManager.Get(query.RID)
		if !found {
			c.String(http.StatusBadRequest, "invalid rid")
			return
		}
		authRequest := authRequestCache.(*model.AuthRequest)

		token, err := app.GitHubOAuthConfig.Exchange(context.Background(), query.Code)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		gitHubApiHttpClient := app.GitHubOAuthConfig.Client(c.Request.Context(), token)
		gitHubApiClient := github.NewClient(gitHubApiHttpClient)
		user, _, err := gitHubApiClient.Users.Get(c.Request.Context(), "")
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		authRequest.GitHubUserID = cast.ToString(user.GetID())
		authRequest.GitHubUserLogin = user.GetLogin()

		authURL, _ := url.Parse(authRequest.AuthURL)
		authURLQuery := authURL.Query()
		authURLQuery.Set(constant.QUERY_KEY_REQUEST_ID, query.RID)
		authURL.RawQuery = authURLQuery.Encode()

		c.Redirect(http.StatusFound, authURL.String())
	}
}

func getAuthResult(app *server.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := model.RequestGetAuthResult{}
		err := c.BindQuery(&query)
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		authRequestCache, found := app.AuthRequestManager.Get(query.RID)
		if !found {
			c.String(http.StatusBadRequest, "invalid rid")
			return
		}
		defer app.AuthRequestManager.Delete(query.RID)
		authRequest := authRequestCache.(*model.AuthRequest)

		c.JSON(
			http.StatusOK,
			model.ResponseGetAuthResult{
				RedirectURI:     authRequest.RedirectURI,
				GitHubUserID:    authRequest.GitHubUserID,
				GitHubUserLogin: authRequest.GitHubUserLogin,
			},
		)
	}
}

func buildRedirectURI(apiBaseUrl, rid string) (string, error) {
	oAuthPageURL, err := url.Parse(apiBaseUrl)
	if err != nil {
		return "", fmt.Errorf("invalid api base url in server config: %w", err)
	}
	oAuthPageURL = oAuthPageURL.JoinPath(constant.ROUTER_GROUP_PATH_OAUTH, constant.ROUTER_PATH_OAUTH_REDIRECT)
	redirectURLQuery := oAuthPageURL.Query()
	redirectURLQuery.Set(constant.QUERY_KEY_REQUEST_ID, rid)
	oAuthPageURL.RawQuery = redirectURLQuery.Encode()
	return oAuthPageURL.String(), nil
}

package router

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	server "github.com/MuXiu1997/traefik-github-oauth-plugin/internal/app/traefik-github-oauth-server"
	"github.com/MuXiu1997/traefik-github-oauth-plugin/internal/app/traefik-github-oauth-server/model"
	"github.com/MuXiu1997/traefik-github-oauth-plugin/internal/pkg/constant"
	"github.com/gin-gonic/gin"
	"github.com/google/go-github/v49/github"
	"github.com/spf13/cast"
	"golang.org/x/oauth2"
)

var (
	ErrInvalidApiBaseURL = fmt.Errorf("invalid api base url")
	ErrInvalidRID        = fmt.Errorf("invalid rid")
	ErrInvalidAuthURL    = fmt.Errorf("invalid auth url")
)

func generateOAuthPageURL(app *server.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		body := model.RequestGenerateOAuthPageURL{}
		err := c.ShouldBindJSON(&body)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.ResponseError{
				Message: fmt.Sprintf("invalid request: %s", err.Error()),
			})
			return
		}

		rid := app.AuthRequestManager.Insert(&model.AuthRequest{
			RedirectURI: body.RedirectURI,
			AuthURL:     body.AuthURL,
		})

		redirectURI, err := buildRedirectURI(app.Config.ApiBaseURL, rid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.ResponseError{
				Message: fmt.Sprintf("[server]%s: %s", err.Error(), app.Config.ApiBaseURL),
			})
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
			return
		}

		authRequest, found := app.AuthRequestManager.Get(query.RID)
		if !found {
			c.String(http.StatusBadRequest, ErrInvalidRID.Error())
			return
		}

		user, err := oAuthCodeToUser(c.Request.Context(), app.GitHubOAuthConfig, query.Code)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		authRequest.GitHubUserID = cast.ToString(user.GetID())
		authRequest.GitHubUserLogin = user.GetLogin()

		authURL, err := url.Parse(authRequest.AuthURL)
		if err != nil {
			c.String(http.StatusInternalServerError, "%s: %s", ErrInvalidAuthURL.Error(), authRequest.AuthURL)
			return
		}
		authURLQuery := authURL.Query()
		authURLQuery.Set(constant.QUERY_KEY_REQUEST_ID, query.RID)
		authURL.RawQuery = authURLQuery.Encode()

		c.Redirect(http.StatusFound, authURL.String())
	}
}

func getAuthResult(app *server.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := model.RequestGetAuthResult{}
		err := c.ShouldBindQuery(&query)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.ResponseError{
				Message: fmt.Sprintf("invalid request: %s", err.Error()),
			})
			return
		}

		authRequest, found := app.AuthRequestManager.Pop(query.RID)
		if !found {
			c.JSON(http.StatusBadRequest, model.ResponseError{
				Message: ErrInvalidRID.Error(),
			})
			return
		}

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

func oAuthCodeToUser(ctx context.Context, oAuthConfig *oauth2.Config, code string) (*github.User, error) {
	token, err := oAuthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}
	gitHubApiHttpClient := oAuthConfig.Client(ctx, token)
	gitHubApiClient := github.NewClient(gitHubApiHttpClient)
	user, _, err := gitHubApiClient.Users.Get(ctx, "")
	if err != nil {
		return nil, err
	}
	return user, nil
}

func buildRedirectURI(apiBaseUrl, rid string) (string, error) {
	redirectURI, err := url.Parse(apiBaseUrl)
	if err != nil {
		return "", ErrInvalidApiBaseURL
	}
	redirectURI = redirectURI.JoinPath(constant.ROUTER_GROUP_PATH_OAUTH, constant.ROUTER_PATH_OAUTH_REDIRECT)
	redirectURLQuery := redirectURI.Query()
	redirectURLQuery.Set(constant.QUERY_KEY_REQUEST_ID, rid)
	redirectURI.RawQuery = redirectURLQuery.Encode()
	return redirectURI.String(), nil
}

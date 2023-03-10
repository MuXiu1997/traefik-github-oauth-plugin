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
		setNoCacheHeaders(c)
		body := model.RequestGenerateOAuthPageURL{}
		err := c.ShouldBindJSON(&body)
		if err != nil {
			app.Logger.Debug().Err(err).Msg("invalid request")
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
			app.Logger.Error().
				Caller().
				Stack().
				Err(err).
				Str("rid", rid).
				Str("api_base_url", app.Config.ApiBaseURL).
				Msg("failed to build redirect uri")
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
		setNoCacheHeaders(c)
		query := model.RequestRedirect{}
		err := c.BindQuery(&query)
		if err != nil {
			app.Logger.Debug().Err(err).Msg("invalid request")
			return
		}

		authRequest, found := app.AuthRequestManager.Get(query.RID)
		if !found {
			app.Logger.Debug().Str("rid", query.RID).Msg("invalid rid")
			c.String(http.StatusBadRequest, ErrInvalidRID.Error())
			return
		}

		user, err := oAuthCodeToUser(c.Request.Context(), app.GitHubOAuthConfig, query.Code)
		if err != nil {
			app.Logger.Error().
				Caller().
				Stack().
				Str("rid", query.RID).
				Str("redirect_uri", authRequest.RedirectURI).
				Str("auth_url", authRequest.AuthURL).
				Err(err).
				Msg("failed to get GitHub user")
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		authRequest.GitHubUserID = cast.ToString(user.GetID())
		authRequest.GitHubUserLogin = user.GetLogin()

		authURL, err := url.Parse(authRequest.AuthURL)
		if err != nil {
			app.Logger.Warn().
				Err(err).
				Str("rid", query.RID).
				Str("auth_url", authRequest.AuthURL).
				Msg("invalid auth url")
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
		setNoCacheHeaders(c)
		query := model.RequestGetAuthResult{}
		err := c.ShouldBindQuery(&query)
		if err != nil {
			app.Logger.Debug().Err(err).Msg("invalid request")
			c.JSON(http.StatusBadRequest, model.ResponseError{
				Message: fmt.Sprintf("invalid request: %s", err.Error()),
			})
			return
		}

		authRequest, found := app.AuthRequestManager.Pop(query.RID)
		if !found {
			app.Logger.Debug().Str("rid", query.RID).Msg("invalid rid")
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
	ctxExchange, cancelExchange := context.WithCancel(ctx)
	defer cancelExchange()
	token, err := oAuthConfig.Exchange(ctxExchange, code)
	if err != nil {
		return nil, err
	}
	ctxClient, cancelClient := context.WithCancel(ctx)
	defer cancelClient()
	gitHubApiHttpClient := oAuthConfig.Client(ctxClient, token)
	gitHubApiClient := github.NewClient(gitHubApiHttpClient)
	ctxGetUser, cancelGetUser := context.WithCancel(ctx)
	defer cancelGetUser()
	user, _, err := gitHubApiClient.Users.Get(ctxGetUser, "")
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

func setNoCacheHeaders(c *gin.Context) {
	c.Header(constant.HTTP_HEADER_CACHE_CONTROL, "no-cache, no-store, must-revalidate, private")
	c.Header(constant.HTTP_HEADER_PRAGMA, "no-cache")
	c.Header(constant.HTTP_HEADER_EXPIRES, "0")
}

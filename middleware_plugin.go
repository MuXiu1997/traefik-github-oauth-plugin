package traefik_github_oauth_plugin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/dghubble/sling"
	"github.com/muxiu1997/traefik-github-oauth-plugin/internal/app/traefik-github-oauth-server/model"
	"github.com/muxiu1997/traefik-github-oauth-plugin/internal/pkg/constant"
	"github.com/muxiu1997/traefik-github-oauth-plugin/internal/pkg/jwt"
	"github.com/scylladb/go-set/strset"
)

const (
	DefaultConfigAuthPath = "/_auth"
)

// Config the middleware configuration.
type Config struct {
	ApiBaseUrl   string          `json:"api_base_url,omitempty"`
	ApiSecretKey string          `json:"api_secret_key,omitempty"`
	AuthPath     string          `json:"auth_path,omitempty"`
	JwtSecretKey string          `json:"jwt_secret_key,omitempty"`
	Whitelist    ConfigWhitelist `json:"whitelist,omitempty"`
}

// ConfigWhitelist the middleware configuration whitelist.
type ConfigWhitelist struct {
	// Ids the GitHub user id list.
	Ids []string `json:"ids,omitempty"`
	// Logins the GitHub user login list.
	Logins []string `json:"logins,omitempty"`
}

// CreateConfig creates the default middleware configuration.
func CreateConfig() *Config {
	return &Config{
		ApiBaseUrl:   "",
		ApiSecretKey: "",
		AuthPath:     DefaultConfigAuthPath,
		JwtSecretKey: getRandomString32(),
		Whitelist: ConfigWhitelist{
			Ids:    []string{},
			Logins: []string{},
		},
	}
}

// TraefikGithubOauthMiddleware the middleware.
type TraefikGithubOauthMiddleware struct {
	ctx  context.Context
	next http.Handler
	name string

	apiBaseUrl        string
	apiSecretKey      string
	authPath          string
	jwtSecretKey      string
	whitelistIdSet    *strset.Set
	whitelistLoginSet *strset.Set
}

var _ http.Handler = (*TraefikGithubOauthMiddleware)(nil)

// New creates a new TraefikGithubOauthMiddleware.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	authPath := config.AuthPath
	if !strings.HasPrefix(authPath, "/") {
		authPath = "/" + authPath
	}
	return &TraefikGithubOauthMiddleware{
		ctx:  ctx,
		next: next,
		name: name,

		apiBaseUrl:        config.ApiBaseUrl,
		apiSecretKey:      config.ApiSecretKey,
		authPath:          authPath,
		jwtSecretKey:      config.JwtSecretKey,
		whitelistIdSet:    strset.New(config.Whitelist.Ids...),
		whitelistLoginSet: strset.New(config.Whitelist.Logins...),
	}, nil
}

// ServeHTTP implements http.Handler.
func (p *TraefikGithubOauthMiddleware) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.URL.Path == p.authPath {
		p.handleAuthRequest(rw, req)
		return
	}
	p.handleRequest(rw, req)
}

// handleRequest
func (p *TraefikGithubOauthMiddleware) handleRequest(rw http.ResponseWriter, req *http.Request) {
	jwtCookie, err := req.Cookie(constant.COOKIE_NAME_JWT)
	if err != nil {
		p.redirectToOAuthPage(rw, req)
		return
	}
	user, err := jwt.ParseTokenString(jwtCookie.Value, p.jwtSecretKey)
	if err != nil {
		p.redirectToOAuthPage(rw, req)
		return
	}
	if !p.whitelistIdSet.Has(user.Id) && !p.whitelistLoginSet.Has(user.Login) {
		http.Error(rw, "not in whitelist", http.StatusForbidden)
		return
	}
	p.next.ServeHTTP(rw, req)
}

// handleAuthRequest
func (p *TraefikGithubOauthMiddleware) handleAuthRequest(rw http.ResponseWriter, req *http.Request) {
	rid := req.URL.Query().Get(constant.QUERY_KEY_REQUEST_ID)
	result, err := p.getAuthResult(rid)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	tokenString, err := jwt.GenerateJwtTokenString(result.GitHubUserID, result.GitHubUserLogin, p.jwtSecretKey)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	http.SetCookie(rw, &http.Cookie{
		Name:     constant.COOKIE_NAME_JWT,
		Value:    tokenString,
		HttpOnly: true,
	})
	http.Redirect(rw, req, result.RedirectURI, http.StatusFound)
}

func (p *TraefikGithubOauthMiddleware) redirectToOAuthPage(rw http.ResponseWriter, req *http.Request) {
	oAuthPageURL, err := p.generateOAuthPageURL(getRawRequestUrl(req), p.getAuthURL(req))
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(rw, req, oAuthPageURL, http.StatusFound)
}

func (p *TraefikGithubOauthMiddleware) generateOAuthPageURL(redirectURI, authURL string) (string, error) {
	reqBody := model.RequestGenerateOAuthPageURL{
		RedirectURI: redirectURI,
		AuthURL:     authURL,
	}
	req := sling.New().Base(p.apiBaseUrl).Post(constant.ROUTER_GROUP_PATH_OAUTH + "/" + constant.ROUTER_PATH_OAUTH_PAGE_URL)
	if 0 < len(p.apiSecretKey) {
		req.Set(constant.HTTP_HEADER_AUTHORIZATION, fmt.Sprintf("%s %s", constant.AUTHORIZATION_PREFIX_TOKEN, p.apiSecretKey))
	}
	var respBody model.ResponseGenerateOAuthPageURL
	var errRespBody model.ResponseError
	_, err := req.BodyJSON(reqBody).Receive(&respBody, &errRespBody)
	if err != nil {
		return "", err
	}
	if 0 < len(errRespBody.Message) {
		return "", fmt.Errorf("rpc failed, message: %s", errRespBody.Message)
	}
	return respBody.OAuthPageURL, nil
}

func (p *TraefikGithubOauthMiddleware) getAuthResult(rid string) (*model.ResponseGetAuthResult, error) {
	req := sling.New().Base(p.apiBaseUrl).Get(constant.ROUTER_GROUP_PATH_OAUTH + "/" + constant.ROUTER_PATH_OAUTH_RESULT)
	if 0 < len(p.apiSecretKey) {
		req.Set(constant.HTTP_HEADER_AUTHORIZATION, fmt.Sprintf("%s %s", constant.AUTHORIZATION_PREFIX_TOKEN, p.apiSecretKey))
	}

	// req.QueryStruct seems to panic in yaegi
	httpRequest, err := req.Request()
	if err != nil {
		return nil, err
	}
	q := httpRequest.URL.Query()
	q.Add(constant.QUERY_KEY_REQUEST_ID, rid)
	httpRequest.URL.RawQuery = q.Encode()

	var respBody model.ResponseGetAuthResult
	var errRespBody model.ResponseError
	_, err = req.Do(httpRequest, &respBody, &errRespBody)
	if err != nil {
		return nil, err
	}
	if 0 < len(errRespBody.Message) {
		return nil, fmt.Errorf("rpc failed, message: %s", errRespBody.Message)
	}
	return &respBody, nil
}

func (p *TraefikGithubOauthMiddleware) getAuthURL(originalReq *http.Request) string {
	var builder strings.Builder
	scheme := "http"
	if originalReq.TLS != nil {
		scheme = "https"
	}
	builder.WriteString(scheme)
	builder.WriteString("://")
	builder.WriteString(originalReq.Host)
	builder.WriteString(p.authPath)
	return builder.String()
}

func getRawRequestUrl(originalReq *http.Request) string {
	var builder strings.Builder
	scheme := "http"
	if originalReq.TLS != nil {
		scheme = "https"
	}
	builder.WriteString(scheme)
	builder.WriteString("://")
	builder.WriteString(originalReq.Host)
	builder.WriteString(originalReq.URL.String())
	return builder.String()
}

func getRandomString32() string {
	randBytes := make([]byte, 16)
	_, _ = rand.Read(randBytes)
	return hex.EncodeToString(randBytes)
}

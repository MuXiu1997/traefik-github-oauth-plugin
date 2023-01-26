package traefik_github_oauth_plugin

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/muxiu1997/traefik-github-oauth-plugin/internal/app/traefik-github-oauth-server/model"
	"github.com/muxiu1997/traefik-github-oauth-plugin/internal/pkg/constant"
	"github.com/muxiu1997/traefik-github-oauth-plugin/internal/pkg/jwt"
	"github.com/scylladb/go-set/strset"
)

const (
	DefaultConfigAuthPath = "/_auth"
)

// Config the plugin configuration.
type Config struct {
	ApiBaseUrl   string          `json:"api_base_url,omitempty"`
	ApiSecretKey string          `json:"api_secret_key,omitempty"`
	AuthPath     string          `json:"auth_path,omitempty"`
	JwtSecretKey string          `json:"jwt_secret_key,omitempty"`
	Whitelist    ConfigWhitelist `json:"whitelist,omitempty"`
}

// ConfigWhitelist the plugin configuration whitelist.
type ConfigWhitelist struct {
	// Ids the GitHub user id list.
	Ids []string `json:"ids,omitempty"`
	// Logins the GitHub user login list.
	Logins []string `json:"logins,omitempty"`
}

// CreateConfig creates the default plugin configuration.
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

// TraefikGithubOauthPlugin the plugin.
type TraefikGithubOauthPlugin struct {
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

var _ http.Handler = (*TraefikGithubOauthPlugin)(nil)

// New creates a new TraefikGithubOauthPlugin.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	authPath := config.AuthPath
	if !strings.HasPrefix(authPath, "/") {
		authPath = "/" + authPath
	}
	return &TraefikGithubOauthPlugin{
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
func (p *TraefikGithubOauthPlugin) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.URL.Path == p.authPath {
		p.handleAuthRequest(rw, req)
		return
	}
	p.handleRequest(rw, req)
}

// handleRequest
func (p *TraefikGithubOauthPlugin) handleRequest(rw http.ResponseWriter, req *http.Request) {
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
func (p *TraefikGithubOauthPlugin) handleAuthRequest(rw http.ResponseWriter, req *http.Request) {
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

func (p *TraefikGithubOauthPlugin) redirectToOAuthPage(rw http.ResponseWriter, req *http.Request) {
	oAuthPageURL, err := p.generateOAuthPageURL(req)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(rw, req, oAuthPageURL, http.StatusFound)
}

func (p *TraefikGithubOauthPlugin) generateOAuthPageURL(originalReq *http.Request) (string, error) {
	var request *http.Request
	{
		requestURL, err := url.Parse(p.apiBaseUrl)
		if err != nil {
			return "", err
		}
		requestURL = requestURL.JoinPath(constant.ROUTER_GROUP_PATH_OAUTH, constant.ROUTER_PATH_OAUTH_PAGE_URL)
		request, err := http.NewRequest(http.MethodPost, requestURL.String(), nil)
		if err != nil {
			return "", err
		}
		request.Header.Add("Content-Type", "application/json")
		if 0 < len(p.apiSecretKey) {
			request.Header.Add(constant.HTTP_HEADER_AUTHORIZATION, fmt.Sprintf("%s %s", constant.AUTHORIZATION_PREFIX_TOKEN, p.apiSecretKey))
		}
		requestBody, err := json.Marshal(model.RequestGenerateOAuthPageURL{
			RedirectURI: getRawRequestUrl(originalReq),
			AuthURL:     p.getAuthURL(originalReq),
		})
		if err != nil {
			return "", err
		}
		request.Body = io.NopCloser(bytes.NewReader(requestBody))
	}

	result := &model.ResponseGenerateOAuthPageURL{}
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", err
	}
	defer func(b io.ReadCloser) {
		_ = b.Close()
	}(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("invalid api secret key")
	}
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("generateOAuthPageURL failed, status code: %d", resp.StatusCode)
	}
	if err = json.NewDecoder(resp.Body).Decode(result); err != nil {
		return "", err
	}
	return result.OAuthPageURL, nil
}

func (p *TraefikGithubOauthPlugin) getAuthResult(rid string) (*model.ResponseGetAuthResult, error) {
	var request *http.Request
	{
		requestURL, err := url.Parse(p.apiBaseUrl)
		if err != nil {
			return nil, err
		}
		requestURL = requestURL.JoinPath(constant.ROUTER_GROUP_PATH_OAUTH, constant.ROUTER_PATH_OAUTH_RESULT)
		requestURLQuery := requestURL.Query()
		requestURLQuery.Set(constant.QUERY_KEY_REQUEST_ID, rid)
		requestURL.RawQuery = requestURLQuery.Encode()
		request, err := http.NewRequest(http.MethodGet, requestURL.String(), nil)
		if err != nil {
			return nil, err
		}
		if 0 < len(p.apiSecretKey) {
			request.Header.Add(constant.HTTP_HEADER_AUTHORIZATION, fmt.Sprintf("%s %s", constant.AUTHORIZATION_PREFIX_TOKEN, p.apiSecretKey))
		}
	}

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer func(b io.ReadCloser) {
		_ = b.Close()
	}(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("invalid api secret key")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("getAuthResult failed, status code: %d", resp.StatusCode)
	}
	result := &model.ResponseGetAuthResult{}
	err = json.NewDecoder(resp.Body).Decode(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (p *TraefikGithubOauthPlugin) getAuthURL(originalReq *http.Request) string {
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

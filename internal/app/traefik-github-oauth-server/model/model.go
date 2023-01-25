package model

type RequestGenerateOAuthPageURL struct {
	RedirectURI string `json:"redirect_uri" binding:"required"`
	AuthURL     string `json:"auth_url" binding:"required"`
}

type ResponseGenerateOAuthPageURL struct {
	OAuthPageURL string `json:"oauth_page_url"`
}

type RequestRedirect struct {
	RID  string `form:"rid" binding:"required"`
	Code string `form:"code" binding:"required"`
}

type RequestGetAuthResult struct {
	RID string `form:"rid" binding:"required"`
}

type ResponseGetAuthResult struct {
	RedirectURI     string `json:"redirect_uri"`
	GitHubUserID    string `json:"github_user_id"`
	GitHubUserLogin string `json:"github_user_login"`
}

type AuthRequest struct {
	RedirectURI     string `json:"redirect_uri"`
	AuthURL         string `json:"auth_url"`
	GitHubUserID    string `json:"github_user_id"`
	GitHubUserLogin string `json:"github_user_login"`
}
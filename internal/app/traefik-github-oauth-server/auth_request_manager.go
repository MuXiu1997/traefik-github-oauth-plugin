package traefik_github_oauth_server

import (
	"github.com/muxiu1997/traefik-github-oauth-plugin/internal/app/traefik-github-oauth-server/model"
	"github.com/patrickmn/go-cache"
	"github.com/rs/xid"
)

type AuthRequestManager struct {
	cache *cache.Cache
}

func NewAuthRequestManager(cache *cache.Cache) *AuthRequestManager {
	return &AuthRequestManager{
		cache: cache,
	}
}

func (m *AuthRequestManager) Insert(aq *model.AuthRequest) string {
	rid := xid.New().String()
	m.cache.SetDefault(rid, aq)
	return rid
}

func (m *AuthRequestManager) Get(rid string) (*model.AuthRequest, bool) {
	authRequest, found := m.cache.Get(rid)
	if !found {
		return nil, false
	}
	return authRequest.(*model.AuthRequest), true
}

func (m *AuthRequestManager) Pop(rid string) (*model.AuthRequest, bool) {
	aq, found := m.Get(rid)
	if found {
		m.cache.Delete(rid)
	}
	return aq, found
}

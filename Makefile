GO?=go
TRAEFIK_GITHUB_OAUTH_SERVER_APP=traefik-github-oauth-server

.PHONY: default
default: build

.PHONY: build
build:
	CGO_ENABLED=0 $(GO) build -o dist/$(TRAEFIK_GITHUB_OAUTH_SERVER_APP) ./cmd/$(TRAEFIK_GITHUB_OAUTH_SERVER_APP)

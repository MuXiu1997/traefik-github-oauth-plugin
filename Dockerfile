FROM alpine:latest

RUN apk add --no-cache ca-certificates && update-ca-certificates

COPY traefik-github-oauth-server /app/traefik-github-oauth-server

WORKDIR /app

EXPOSE 80

ENTRYPOINT ["/app/traefik-github-oauth-server"]

FROM alpine:latest

RUN apk add --no-cache ca-certificates && update-ca-certificates

COPY /dist/traefik-github-oauth-server /app/traefik-github-oauth-server

WORKDIR /app

EXPOSE 80

CMD ["/app/traefik-github-oauth-server"]

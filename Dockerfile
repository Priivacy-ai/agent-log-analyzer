FROM node:22-alpine AS webbuild
WORKDIR /src
COPY package.json package-lock.json vite.config.mjs ./
COPY scripts/build-web-assets.mjs ./scripts/build-web-assets.mjs
COPY web ./web
RUN npm ci
RUN npm run build:web

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
COPY testdata ./testdata
COPY web ./web
COPY docs ./docs
RUN go test ./...
RUN go build -o /out/api ./cmd/api
RUN go build -o /out/worker ./cmd/worker
RUN go build -o /out/sweeper ./cmd/sweeper
RUN go build -o /out/email-events ./cmd/email-events

FROM alpine:3.22
RUN adduser -D -H appuser
WORKDIR /app
COPY --from=build /out/api /usr/local/bin/claude-analyzer-api
COPY --from=build /out/worker /usr/local/bin/claude-analyzer-worker
COPY --from=build /out/sweeper /usr/local/bin/claude-analyzer-sweeper
COPY --from=build /out/email-events /usr/local/bin/claude-analyzer-email-events
COPY web ./web
COPY --from=webbuild /src/web-dist ./web
COPY docs ./web/docs
RUN mkdir -p /data && chown -R appuser:appuser /data
USER appuser
EXPOSE 8080
CMD ["claude-analyzer-api"]

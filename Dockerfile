FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN go test ./...
RUN go build -o /out/api ./cmd/api
RUN go build -o /out/worker ./cmd/worker

FROM alpine:3.22
RUN adduser -D -H appuser
WORKDIR /app
COPY --from=build /out/api /usr/local/bin/claude-analyzer-api
COPY --from=build /out/worker /usr/local/bin/claude-analyzer-worker
COPY web ./web
USER appuser
EXPOSE 8080
CMD ["claude-analyzer-api"]


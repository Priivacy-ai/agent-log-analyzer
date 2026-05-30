//go:build lambda

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/priivacy-ai/agent-log-analyzer/internal/backend"
)

var lambdaHandler http.Handler

func main() {
	store, err := backend.NewAPIStore()
	if err != nil {
		slog.Error("store init failed", "error", err)
		os.Exit(1)
	}
	lambdaHandler = logRequests(buildMux(store), store)
	lambda.Start(handleAPIGatewayV2)
}

func handleAPIGatewayV2(ctx context.Context, event events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	req, err := requestFromAPIGatewayV2(ctx, event)
	if err != nil {
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusBadRequest,
			Headers:    map[string]string{"content-type": "application/json"},
			Body:       `{"error":"invalid request"}`,
		}, nil
	}
	recorder := &lambdaResponseRecorder{headers: make(http.Header), statusCode: http.StatusOK}
	lambdaHandler.ServeHTTP(recorder, req)
	return recorder.response(), nil
}

func requestFromAPIGatewayV2(ctx context.Context, event events.APIGatewayV2HTTPRequest) (*http.Request, error) {
	body := []byte(event.Body)
	if event.IsBase64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(event.Body)
		if err != nil {
			return nil, err
		}
		body = decoded
	}
	rawPath := event.RawPath
	if rawPath == "" {
		rawPath = event.RequestContext.HTTP.Path
	}
	rawQuery := event.RawQueryString
	target := rawPath
	if rawQuery != "" {
		target += "?" + rawQuery
	}
	req, err := http.NewRequestWithContext(ctx, event.RequestContext.HTTP.Method, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.URL.RawQuery = rawQuery
	req.RequestURI = target
	req.RemoteAddr = event.RequestContext.HTTP.SourceIP
	for key, value := range event.Headers {
		req.Header.Set(key, value)
	}
	if req.Host == "" {
		req.Host = event.Headers["host"]
	}
	if req.Host == "" {
		req.Host = event.RequestContext.DomainName
	}
	if req.URL.Scheme == "" {
		req.URL.Scheme = forwardedProto(req.Header)
	}
	if req.URL.Host == "" {
		req.URL.Host = req.Host
	}
	for _, cookie := range event.Cookies {
		req.Header.Add("Cookie", cookie)
	}
	return req, nil
}

func forwardedProto(header http.Header) string {
	if proto := header.Get("x-forwarded-proto"); proto != "" {
		return strings.Split(proto, ",")[0]
	}
	return "https"
}

type lambdaResponseRecorder struct {
	headers    http.Header
	body       bytes.Buffer
	statusCode int
}

func (r *lambdaResponseRecorder) Header() http.Header {
	return r.headers
}

func (r *lambdaResponseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}

func (r *lambdaResponseRecorder) Write(data []byte) (int, error) {
	return r.body.Write(data)
}

func (r *lambdaResponseRecorder) response() events.APIGatewayV2HTTPResponse {
	headers := map[string]string{}
	cookies := []string{}
	for key, values := range r.headers {
		if strings.EqualFold(key, "Set-Cookie") {
			cookies = append(cookies, values...)
			continue
		}
		if len(values) > 0 {
			headers[key] = strings.Join(values, ", ")
		}
	}
	bodyBytes := r.body.Bytes()
	body := string(bodyBytes)
	isBinary := responseIsBinary(headers)
	if isBinary {
		body = base64.StdEncoding.EncodeToString(bodyBytes)
	}
	return events.APIGatewayV2HTTPResponse{
		StatusCode:      r.statusCode,
		Headers:         headers,
		Cookies:         cookies,
		Body:            body,
		IsBase64Encoded: isBinary,
	}
}

func responseIsBinary(headers map[string]string) bool {
	contentType := strings.ToLower(headers["Content-Type"])
	if contentType == "" {
		contentType = strings.ToLower(headers["content-type"])
	}
	if strings.HasPrefix(contentType, "text/") ||
		strings.Contains(contentType, "json") ||
		strings.Contains(contentType, "javascript") ||
		strings.Contains(contentType, "xml") ||
		strings.Contains(contentType, "markdown") {
		return false
	}
	if contentType == "" {
		return false
	}
	return true
}

var _ http.ResponseWriter = (*lambdaResponseRecorder)(nil)
var _ io.Writer = (*lambdaResponseRecorder)(nil)

package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type Config struct {
	URL     string
	Timeout time.Duration
}

type Gateway struct {
	endpoint string
	client   *http.Client
}

type TransportError struct {
	Err error
}

func (e TransportError) Error() string {
	return fmt.Sprintf("push transport: %v", e.Err)
}

func (e TransportError) Unwrap() error {
	return e.Err
}

type StatusError struct {
	StatusCode int
}

func (e StatusError) Error() string {
	return fmt.Sprintf("push returned status %d", e.StatusCode)
}

func New(config Config) (*Gateway, error) {
	baseURL, err := url.Parse(config.URL)
	if err != nil {
		return nil, fmt.Errorf("parse push URL: %w", err)
	}
	if baseURL.Scheme == "" || baseURL.Host == "" {
		return nil, fmt.Errorf("parse push URL: absolute URL is required")
	}
	endpoint := baseURL.ResolveReference(&url.URL{Path: "/api/v1/push"}).String()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	return &Gateway{
		endpoint: endpoint,
		client: &http.Client{
			Timeout:   config.Timeout,
			Transport: otelhttp.NewTransport(transport),
		},
	}, nil
}

func (g *Gateway) RequestPosition(ctx context.Context, tripID, driverID uuid.UUID) error {
	payload := struct {
		RequestID   string            `json:"request_id"`
		RecipientID uuid.UUID         `json:"recipient_id"`
		Kind        string            `json:"kind"`
		Data        map[string]string `json:"data"`
	}{
		RequestID:   uuid.NewString(),
		RecipientID: driverID,
		Kind:        "REQUEST_POSITION",
		Data:        map[string]string{"trip_id": tripID.String()},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal push request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create push request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	response, err := g.client.Do(req)
	if err != nil {
		return TransportError{Err: err}
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	if response.StatusCode != http.StatusAccepted {
		return StatusError{StatusCode: response.StatusCode}
	}
	return nil
}

func (g *Gateway) Close() {
	g.client.CloseIdleConnections()
}

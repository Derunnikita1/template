package push

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

const testPushURL = "http://push.test"

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestRequestPosition(t *testing.T) {
	gateway, err := New(Config{URL: testPushURL, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	gateway.client.Transport = roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.Path != "/api/v1/push" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusAccepted,
			Body:       io.NopCloser(strings.NewReader("{}")),
			Header:     make(http.Header),
		}, nil
	})
	if err := gateway.RequestPosition(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
}

func TestRequestPositionDistinguishesStatusError(t *testing.T) {
	gateway, err := New(Config{URL: testPushURL, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	gateway.client.Transport = roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       io.NopCloser(strings.NewReader("{}")),
			Header:     make(http.Header),
		}, nil
	})
	err = gateway.RequestPosition(context.Background(), uuid.New(), uuid.New())
	var statusError StatusError
	if !errors.As(err, &statusError) || statusError.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("error = %v", err)
	}
}

func TestRequestPositionDistinguishesTransportError(t *testing.T) {
	gateway, err := New(Config{URL: testPushURL, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	gateway.client.Transport = roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("connection failed")
	})
	err = gateway.RequestPosition(context.Background(), uuid.New(), uuid.New())
	var transportError TransportError
	if !errors.As(err, &transportError) {
		t.Fatalf("error = %v", err)
	}
}

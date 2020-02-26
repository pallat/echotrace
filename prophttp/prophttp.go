package prophttp

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/plugin/httptrace"
	"google.golang.org/grpc/codes"
)

// NewRequestWithContext returns *http.Request with tracing context
func NewRequestWithContext(ctx context.Context, method string, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	httptrace.W3C(ctx, req)
	httptrace.Inject(ctx, req)

	return req, nil
}

// Client struct composition *http.Client with trace.Tracer
type Client struct {
	*http.Client
	tr trace.Tracer
}

func NewCustomeClientWithContext(name string, maxIdleConns int, timeout time.Duration, insec bool) *Client {
	tr := global.TraceProvider().Tracer(name)

	return &Client{
		Client: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:    maxIdleConns,
				TLSClientConfig: &tls.Config{InsecureSkipVerify: insec},
			},
			Timeout: timeout,
		},
		tr: tr,
	}
}

const (
	defaultMaxIdleConns       = 20
	defaultTimeout            = 5 * time.Second
	defaultInsucureSkipVerify = false
)

// NewClientWithContext receives trace name then returns *Client
func NewClientWithContext(name string) *Client {
	tr := global.TraceProvider().Tracer(name)

	return &Client{
		Client: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:    defaultMaxIdleConns,
				TLSClientConfig: &tls.Config{InsecureSkipVerify: defaultInsucureSkipVerify},
			},
			Timeout: defaultTimeout,
		},
		tr: tr,
	}
}

// Do extend http.Client.Do to do tracing
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	var res *http.Response
	var err error

	c.tr.WithSpan(ctx, "client-op",
		func(ctx context.Context) error {
			res, err = c.Client.Do(req)
			trace.SpanFromContext(ctx).SetStatus(codes.OK)
			return err
		})

	return res, errors.Wrap(err, "tracing client do")
}

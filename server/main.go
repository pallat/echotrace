package main

import (
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pallat/echotrace/prophttp"

	"go.opentelemetry.io/otel/api/distributedcontext"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/exporter/trace/stdout"
	"go.opentelemetry.io/otel/plugin/httptrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func initTracer() {
	// Create stdout exporter to be able to retrieve
	// the collected spans.
	exporter, err := stdout.NewExporter(stdout.Options{PrettyPrint: true})
	if err != nil {
		log.Fatal(err)
	}

	// For the demonstration, use sdktrace.AlwaysSample sampler to sample all traces.
	// In a production application, use sdktrace.ProbabilitySampler with a desired probability.
	tp, err := sdktrace.NewProvider(sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
		sdktrace.WithSyncer(exporter))
	if err != nil {
		log.Fatal(err)
	}
	global.SetTraceProvider(tp)
}

func main() {
	initTracer()

	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(TraceMiddleware)

	// Routes
	e.GET("/", hello)
	e.GET("/handler", Handler)

	// Start server
	e.Logger.Fatal(e.Start(":1323"))

}

func hello(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, World!")
}

func TraceMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	tr := global.TraceProvider().Tracer("example/server")
	return func(c echo.Context) error {
		req := c.Request()
		attrs, entries, spanCtx := httptrace.Extract(req.Context(), req)

		req = req.WithContext(distributedcontext.WithMap(req.Context(), distributedcontext.NewMap(distributedcontext.MapUpdate{
			MultiKV: entries,
		})))

		_, span := tr.Start(
			req.Context(),
			"server",
			trace.WithAttributes(attrs...),
			trace.ChildOf(spanCtx),
		)
		defer span.End()

		c.SetRequest(req.WithContext(trace.ContextWithSpan(req.Context(), span)))

		return next(c)
	}
}

func Handler(c echo.Context) error {
	client := prophttp.NewClientWithContext("example/server")
	req, _ := prophttp.NewRequestWithContext(c.Request().Context(), http.MethodGet, "http://localhost:7777/", nil)

	client.Do(c.Request().Context(), req)
	return nil
}

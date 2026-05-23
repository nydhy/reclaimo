package telemetry

import (
	"context"
	"log"
	"sync/atomic"

	ddtrace "gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type Config struct {
	Enabled   bool
	Service   string
	AgentAddr string
	Env       string
}

type Span struct {
	span ddtrace.Span
}

var enabled atomic.Bool

func Start(cfg Config) func() {
	if !cfg.Enabled {
		enabled.Store(false)
		return func() {}
	}

	if cfg.Service == "" {
		cfg.Service = "reclaimo-api"
	}
	if cfg.AgentAddr == "" {
		cfg.AgentAddr = "127.0.0.1:8126"
	}
	if cfg.Env == "" {
		cfg.Env = "local"
	}

	tracer.Start(
		tracer.WithService(cfg.Service),
		tracer.WithAgentAddr(cfg.AgentAddr),
		tracer.WithEnv(cfg.Env),
		tracer.WithRuntimeMetrics(),
	)
	enabled.Store(true)
	log.Printf("datadog tracing enabled: service=%s agent=%s env=%s", cfg.Service, cfg.AgentAddr, cfg.Env)

	return func() {
		tracer.Stop()
		enabled.Store(false)
	}
}

func StartSpan(ctx context.Context, name string, tags map[string]any) (context.Context, *Span) {
	if !enabled.Load() {
		return ctx, &Span{}
	}

	opts := make([]tracer.StartSpanOption, 0, len(tags))
	for key, value := range tags {
		opts = append(opts, tracer.Tag(key, value))
	}

	span, spanCtx := tracer.StartSpanFromContext(ctx, name, opts...)
	return spanCtx, &Span{span: span}
}

func (s *Span) SetTag(key string, value any) {
	if s == nil || s.span == nil {
		return
	}
	s.span.SetTag(key, value)
}

func (s *Span) Finish(err error) {
	if s == nil || s.span == nil {
		return
	}
	if err != nil {
		s.span.Finish(tracer.WithError(err))
		return
	}
	s.span.Finish()
}

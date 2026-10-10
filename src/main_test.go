package main

import (
	"context"
	"log/slog"
	"testing"

	"webrtc-load-engine/src/scenario"
)

type noOpLogger struct{}

func (noOpLogger) Enabled(context.Context, slog.Level) bool  { return false }
func (noOpLogger) Handle(context.Context, slog.Record) error { return nil }
func (n noOpLogger) WithAttrs([]slog.Attr) slog.Handler      { return n }
func (n noOpLogger) WithGroup(string) slog.Handler           { return n }

func TestMainEngineStarter_RejectsMissingPlatformConfig(t *testing.T) {
	starter := NewMainEngineStarter(slog.New(noOpLogger{}))

	engine, err := starter.StartExecution("s1", scenario.TestPlan{Participants: 1, DurationSeconds: 1}, nil, func(string, string) {})
	if err == nil {
		t.Fatal("expected an error when platform_config is missing, got nil")
	}
	if engine != nil {
		t.Errorf("expected no engine on error, got %v", engine)
	}
}

func TestMainEngineStarter_ConfiguresJitsiAdapterFromPlatformConfig(t *testing.T) {
	starter := NewMainEngineStarter(slog.New(noOpLogger{}))
	tp := scenario.TestPlan{
		Participants:    1,
		DurationSeconds: 1,
		PlatformConfig:  map[string]any{"xmpp_domain": "meet.example.org", "room_name": "load1"},
	}

	engine, err := starter.StartExecution("s1", tp, nil, func(string, string) {})
	if err != nil {
		t.Fatalf("expected engine to start with valid platform_config, got %v", err)
	}
	if engine == nil {
		t.Fatal("expected an engine, got nil")
	}
}

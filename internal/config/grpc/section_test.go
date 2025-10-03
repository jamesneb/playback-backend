package grpc

import (
	"testing"
)

type emptyResolver struct{}

func (e emptyResolver) Resolve(key string) (string, bool) {
	return "", false
}

func TestDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.Port != DEFAULT_GRPC_SERVER_PORT {
		t.Errorf("Defaults() Port = %d, want %d", cfg.Port, DEFAULT_GRPC_SERVER_PORT)
	}
	if cfg.Port != 4317 {
		t.Errorf("Defaults() Port = %d, want 4317", cfg.Port)
	}
}

func TestFromResolver_EmptyResolver_ShouldUseDefaults(t *testing.T) {
	r := emptyResolver{}
	cfg, err := FromResolver(r)
	if err != nil {
		t.Fatalf("FromResolver() error = %v, want nil", err)
	}
	if cfg.Port != DEFAULT_GRPC_SERVER_PORT {
		t.Errorf("FromResolver() Port = %d, want %d", cfg.Port, DEFAULT_GRPC_SERVER_PORT)
	}
}

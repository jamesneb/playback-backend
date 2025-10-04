package config

import (
	"context"
	"testing"

	"github.com/jamesneb/playback-backend/internal/config/grpc"
	"github.com/jamesneb/playback-backend/internal/config/provider"
	"github.com/stretchr/testify/assert"
)

type FakeProvider struct {
	name string
	data map[string]string
}

func (fp *FakeProvider) Load(ctx context.Context) (map[string]string, error) {
	if fp.data == nil {
		return make(map[string]string), nil
	}
	return fp.data, nil
}

func (fp *FakeProvider) Name() string {
	if fp.name == "" {
		return "fake"
	}
	return fp.name
}

func fakeProvider() provider.Provider {
	return &FakeProvider{name: "fake"}
}

func TestManager_InitialLoadProducesDefaultsForGRPCServer(t *testing.T) {
	prov := fakeProvider()
	m, err := NewManager(context.Background(), prov)
	if err != nil {
		t.Fatal(err)
	}

	got := m.Snapshot().GRPCServer
	assert.Equal(t, grpc.DEFAULT_GRPC_SERVER_PORT, got.Port)
}

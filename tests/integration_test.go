package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Laotree/uuid-compat-bench/internal/benchmark"
	"github.com/Laotree/uuid-compat-bench/internal/clickhouse"
	"github.com/Laotree/uuid-compat-bench/internal/config"
)

func chClient(t *testing.T) *clickhouse.Client {
	t.Helper()
	if os.Getenv("CLICKHOUSE_PASSWORD") == "" {
		t.Skip("CLICKHOUSE_PASSWORD not set; skipping ClickHouse integration test")
	}
	cfg := config.Default()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := clickhouse.Connect(ctx, cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func TestCompatibilityAllScenarios(t *testing.T) {
	client := chClient(t)
	ctx := context.Background()
	cfg := config.Default()
	cfg.Rows = 1000
	cfg.Table = "uuid_compat_it"

	res, err := benchmark.RunCompatibility(ctx, client, cfg)
	if err != nil {
		t.Fatalf("compat: %v", err)
	}
	for _, sc := range res.Scenarios {
		if sc.Native.Inserted != 1000 {
			t.Errorf("%s: inserted=%d, want 1000", sc.Name, sc.Native.Inserted)
		}
		if !sc.Native.GenOK() || !sc.Bridge.Pass() {
			t.Errorf("%s: native counters bad: %+v bridge: %+v", sc.Name, sc.Native, sc.Bridge)
		}
	}
}

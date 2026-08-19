package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultHost           = "localhost"
	DefaultPort           = 9000
	DefaultDatabase       = "default"
	DefaultUsername       = "default"
	DefaultRows           = 1_000_000
	DefaultTable          = "uuid_compat_test"
	DefaultWarmup         = 10 * time.Second
	DefaultDuration       = 30 * time.Second
	DefaultIterations     = 3
	DefaultWarnRegression = 5.0
	DefaultFailRegression = 10.0
)

type Config struct {
	Host           string
	Port           int
	Database       string
	Username       string
	Password       string
	Secure         bool
	Rows           int
	Concurrency    []int
	Warmup         time.Duration
	Duration       time.Duration
	Iterations     int
	Table          string
	UUIDVersion    string
	WarnRegression float64
	FailRegression float64
	Output         string
}

func Default() Config {
	return Config{
		Host:           envString("CLICKHOUSE_HOST", DefaultHost),
		Port:           envInt("CLICKHOUSE_PORT", DefaultPort),
		Database:       envString("CLICKHOUSE_DATABASE", DefaultDatabase),
		Username:       envString("CLICKHOUSE_USERNAME", DefaultUsername),
		Password:       envString("CLICKHOUSE_PASSWORD", ""),
		Rows:           DefaultRows,
		Concurrency:    []int{1, 2, 4, 8, 16, 32, 64, 128},
		Warmup:         DefaultWarmup,
		Duration:       DefaultDuration,
		Iterations:     DefaultIterations,
		Table:          DefaultTable,
		UUIDVersion:    "v4",
		WarnRegression: DefaultWarnRegression,
		FailRegression: DefaultFailRegression,
		Output:         "text",
	}
}

func ParseConcurrency(s string) ([]int, error) {
	parts := strings.Split(s, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func envString(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

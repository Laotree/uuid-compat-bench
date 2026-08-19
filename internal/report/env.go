package report

import (
	"fmt"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/Laotree/uuid-compat-bench/internal/config"
)

type Environment struct {
	GoVersion         string
	OS                string
	Arch              string
	CPU               string
	DriverVersion     string
	GoogleUUIDVersion string
	ClickHouseVersion string
	Rows              int
	Concurrency       []int
	Duration          time.Duration
	Warmup            time.Duration
	NetworkMode       string
}

func CollectEnv(cfg config.Config, chVersion string) Environment {
	env := Environment{
		GoVersion:         runtime.Version(),
		OS:                runtime.GOOS,
		Arch:              runtime.GOARCH,
		CPU:               cpuName(),
		ClickHouseVersion: chVersion,
		Rows:              cfg.Rows,
		Concurrency:       cfg.Concurrency,
		Duration:          cfg.Duration,
		Warmup:            cfg.Warmup,
		NetworkMode:       "local",
	}
	if cfg.Host != "localhost" && cfg.Host != "127.0.0.1" && cfg.Host != "::1" {
		env.NetworkMode = "remote"
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, m := range bi.Deps {
			version := m.Version
			if m.Replace != nil {
				version = m.Replace.Version
			}
			switch m.Path {
			case "github.com/ClickHouse/clickhouse-go/v2":
				env.DriverVersion = version
			case "github.com/google/uuid":
				env.GoogleUUIDVersion = version
			}
		}
	}
	return env
}

func cpuName() string {
	var args []string
	switch runtime.GOOS {
	case "darwin":
		args = []string{"sysctl", "-n", "machdep.cpu.brand_string"}
	case "linux":
		args = []string{"sh", "-c", "grep -m1 'model name' /proc/cpuinfo | cut -d: -f2"}
	default:
		return runtime.GOOS + "/" + runtime.GOARCH
	}
	out, err := exec.Command(args[0], args[1:]...).Output()
	if err != nil {
		return runtime.GOOS + "/" + runtime.GOARCH
	}
	return strings.TrimSpace(string(out))
}

func (e Environment) ConcurrencyDisplay() string {
	parts := make([]string, len(e.Concurrency))
	for i, c := range e.Concurrency {
		parts[i] = fmt.Sprint(c)
	}
	return strings.Join(parts, ",")
}

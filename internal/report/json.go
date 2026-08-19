package report

import (
	"encoding/json"
	"fmt"

	"github.com/Laotree/uuid-compat-bench/internal/benchmark"
)

type compatJSON struct {
	GoogleToGoogle bool `json:"google_to_google"`
	StdlibToStdlib bool `json:"stdlib_to_stdlib"`
	GoogleToStdlib bool `json:"google_to_stdlib"`
	StdlibToGoogle bool `json:"stdlib_to_google"`
}

type perfJSON struct {
	BaselineRowsPerSecond  float64  `json:"baseline_rows_per_second,omitempty"`
	CandidateRowsPerSecond float64  `json:"candidate_rows_per_second,omitempty"`
	RegressionPercent      float64  `json:"regression_percent,omitempty"`
	Saturation             []string `json:"saturation,omitempty"`
}

type verdictJSON struct {
	Compatibility compatJSON `json:"compatibility"`
	Performance   perfJSON   `json:"performance"`
	Environment   envJSON    `json:"environment,omitempty"`
	Verdict       string     `json:"verdict"`
}

type envJSON struct {
	GoVersion         string `json:"go_version"`
	OS                string `json:"os"`
	Arch              string `json:"arch"`
	CPU               string `json:"cpu"`
	ClickHouseVersion string `json:"clickhouse_version"`
	DriverVersion     string `json:"driver_version"`
	GoogleUUIDVersion string `json:"google_uuid_version"`
	Rows              int    `json:"rows"`
	Concurrency       []int  `json:"concurrency"`
	Duration          string `json:"duration"`
	Warmup            string `json:"warmup"`
	NetworkMode       string `json:"network_mode"`
}

func CompatJSON(res *benchmark.CompatResult, env Environment) string {
	sc := map[string]*benchmark.ScenarioResult{}
	for i := range res.Scenarios {
		sc[res.Scenarios[i].Name] = &res.Scenarios[i]
	}
	pick := func(name string) bool {
		s, ok := sc[name]
		return ok && s.Pass()
	}
	out := verdictJSON{
		Compatibility: compatJSON{
			GoogleToGoogle: pick("google -> google"),
			StdlibToStdlib: pick("stdlib -> stdlib"),
			GoogleToStdlib: pick("google -> stdlib"),
			StdlibToGoogle: pick("stdlib -> google"),
		},
		Environment: toEnvJSON(env),
	}
	out.Verdict = "PASS"
	if !res.AllPass() {
		out.Verdict = "FAIL"
	}
	return marshal(&out)
}

func PerfJSON(res *benchmark.PerfResult, env Environment) string {
	out := verdictJSON{
		Performance: perfJSON{
			BaselineRowsPerSecond:  res.MaxRowsPerSec("google -> google"),
			CandidateRowsPerSecond: res.MaxRowsPerSec("stdlib -> stdlib"),
			RegressionPercent:      res.RegressionPercent(),
			Saturation:             res.SaturationNotes,
		},
		Environment: toEnvJSON(env),
		Verdict:     res.Verdict(),
	}
	return marshal(&out)
}

func FullJSON(compat *benchmark.CompatResult, perf *benchmark.PerfResult, env Environment) string {
	sc := map[string]*benchmark.ScenarioResult{}
	for i := range compat.Scenarios {
		sc[compat.Scenarios[i].Name] = &compat.Scenarios[i]
	}
	pick := func(name string) bool {
		s, ok := sc[name]
		return ok && s.Pass()
	}
	out := verdictJSON{
		Compatibility: compatJSON{
			GoogleToGoogle: pick("google -> google"),
			StdlibToStdlib: pick("stdlib -> stdlib"),
			GoogleToStdlib: pick("google -> stdlib"),
			StdlibToGoogle: pick("stdlib -> google"),
		},
		Performance: perfJSON{
			BaselineRowsPerSecond:  perf.MaxRowsPerSec("google -> google"),
			CandidateRowsPerSecond: perf.MaxRowsPerSec("stdlib -> stdlib"),
			RegressionPercent:      perf.RegressionPercent(),
			Saturation:             perf.SaturationNotes,
		},
		Environment: toEnvJSON(env),
	}
	compatOK := compat.AllPass()
	perfOK := perf.AllPass()
	switch {
	case !compatOK || !perfOK:
		out.Verdict = "FAIL"
	case perf.Verdict() == "WARNING":
		out.Verdict = "WARNING"
	default:
		out.Verdict = "PASS"
	}
	return marshal(&out)
}

func toEnvJSON(env Environment) envJSON {
	return envJSON{
		GoVersion:         env.GoVersion,
		OS:                env.OS,
		Arch:              env.Arch,
		CPU:               env.CPU,
		ClickHouseVersion: env.ClickHouseVersion,
		DriverVersion:     env.DriverVersion,
		GoogleUUIDVersion: env.GoogleUUIDVersion,
		Rows:              env.Rows,
		Concurrency:       env.Concurrency,
		Duration:          env.Duration.String(),
		Warmup:            env.Warmup.String(),
		NetworkMode:       env.NetworkMode,
	}
}

func marshal(v any) string {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error())
	}
	return string(out)
}

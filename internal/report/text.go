package report

import (
	"fmt"
	"strings"

	"github.com/Laotree/uuid-compat-bench/internal/benchmark"
)

func CompatText(res *benchmark.CompatResult, env Environment) string {
	var b strings.Builder
	writeEnvironment(&b, env)
	b.WriteString("Compatibility\n-------------\n\n")
	for _, sc := range res.Scenarios {
		verdict := "FAIL"
		if sc.Pass() {
			verdict = "PASS"
		}
		native := countersStatus(sc.Native)
		bridge := countersStatus(sc.Bridge)
		fmt.Fprintf(&b, "%-18s %-5s native[%s] bridge[%s]\n", sc.Name, verdict, native, bridge)
		fmt.Fprintf(&b, "  native : generated=%d inserted=%d read=%d matched=%d mismatched=%d ins_err=%d read_err=%d decode_err=%d\n",
			sc.Native.Generated, sc.Native.Inserted, sc.Native.Read, sc.Native.Matched,
			sc.Native.Mismatched, sc.Native.InsertErrors, sc.Native.ReadErrors, sc.Native.DecodeErrors)
		fmt.Fprintf(&b, "  bridge : inserted=%d read=%d matched=%d mismatched=%d ins_err=%d read_err=%d decode_err=%d\n",
			sc.Bridge.Inserted, sc.Bridge.Read, sc.Bridge.Matched, sc.Bridge.Mismatched,
			sc.Bridge.InsertErrors, sc.Bridge.ReadErrors, sc.Bridge.DecodeErrors)
		if sc.Note != "" {
			fmt.Fprintf(&b, "  %s\n", sc.Note)
		}
	}
	return b.String()
}

func countersStatus(c benchmark.Counters) string {
	if c.Pass() {
		return "PASS"
	}
	return "FAIL"
}

func PerfText(res *benchmark.PerfResult, env Environment) string {
	var b strings.Builder
	writeEnvironment(&b, env)
	b.WriteString("Throughput\n----------\n\n")
	names := scenarioNames(res)
	header := "Concurrency  "
	widths := []int{}
	for _, n := range names {
		label := n
		if len(label) > 12 {
			label = n[:12]
		}
		widths = append(widths, len(label)+2)
		header += pad(label, widths[len(widths)-1])
	}
	b.WriteString(header + "\n")

	for _, c := range res.Concurrency {
		line := fmt.Sprintf("%-12d  ", c)
		for j, n := range names {
			sc := res.Scenario(n)
			v := findConcurrency(sc, c)
			line += pad(formatRate(v), widths[j])
		}
		b.WriteString(line + "\n")
	}
	for _, note := range res.SaturationNotes {
		b.WriteString(note + "\n")
	}

	reg := res.RegressionPercent()
	base := "0"
	if bl := res.Scenario("google -> google"); bl != nil {
		base = fmt.Sprintf("%.2fM", bl.MaxRowsPerSec()/1e6)
	}
	cand := "0"
	if cd := res.Scenario("stdlib -> stdlib"); cd != nil {
		cand = fmt.Sprintf("%.2fM", cd.MaxRowsPerSec()/1e6)
	}
	fmt.Fprintf(&b, "\nRegression\n----------\n\nstdlib vs google baseline (%s vs %s):\n  %.2f%%\n",
		base, cand, reg)
	fmt.Fprintf(&b, "  thresholds: warn > %.0f%%, fail > %.0f%%\n", res.WarnRegression, res.FailRegression)
	b.WriteString("\nVerdict\n-------\n\n")
	b.WriteString(res.Verdict() + "\n")
	if res.Verdict() == "WARNING" {
		b.WriteString("\nnote: performance regression exceeds warning threshold\n")
	}
	return b.String()
}

func findConcurrency(sc *benchmark.ScenarioPerf, c int) float64 {
	if sc == nil {
		return 0
	}
	for _, pac := range sc.ConcurrencyResults {
		if pac.Concurrency == c {
			return pac.InsertRowsPerSec.Median
		}
	}
	return 0
}

func scenarioNames(res *benchmark.PerfResult) []string {
	names := []string{}
	for _, s := range res.Scenarios {
		names = append(names, s.Name)
	}
	return names
}

func pad(s string, width int) string {
	if len(s) >= width {
		return s + " "
	}
	return s + strings.Repeat(" ", width-len(s))
}

func formatRate(v float64) string {
	if v >= 1e6 {
		return fmt.Sprintf("%.2fM/s", v/1e6)
	}
	if v >= 1e3 {
		return fmt.Sprintf("%.1fK/s", v/1e3)
	}
	return fmt.Sprintf("%.0f/s", v)
}

func FullText(compat *benchmark.CompatResult, perf *benchmark.PerfResult, env Environment) string {
	var b strings.Builder
	b.WriteString("UUID Compatibility & Throughput Benchmark\n")
	b.WriteString("==========================================\n\n")
	writeEnvironment(&b, env)
	b.WriteString(CompatText(compat, env))
	b.WriteString("\n")
	b.WriteString(PerfText(perf, env))
	b.WriteString("\nOverall Verdict\n---------------\n\n")
	overall := "PASS"
	if !compat.AllPass() || !perf.AllPass() {
		overall = "FAIL"
	}
	b.WriteString(overall + "\n")
	return b.String()
}

func writeEnvironment(b *strings.Builder, env Environment) {
	fmt.Fprintf(b, "Environment:\n")
	fmt.Fprintf(b, "  go: %s %s/%s\n", env.GoVersion, env.OS, env.Arch)
	fmt.Fprintf(b, "  cpu: %s\n", env.CPU)
	fmt.Fprintf(b, "  clickhouse: %s\n", env.ClickHouseVersion)
	fmt.Fprintf(b, "  clickhouse-go: %s\n", env.DriverVersion)
	fmt.Fprintf(b, "  google/uuid: %s\n", env.GoogleUUIDVersion)
	fmt.Fprintf(b, "  network mode: %s\n", env.NetworkMode)
	fmt.Fprintf(b, "  rows: %d\n", env.Rows)
	fmt.Fprintf(b, "  concurrency: %s\n", env.ConcurrencyDisplay())
	fmt.Fprintf(b, "  duration: %s  warmup: %s\n\n", env.Duration, env.Warmup)
}

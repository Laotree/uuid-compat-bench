package benchmark

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Laotree/uuid-compat-bench/internal/clickhouse"
	"github.com/Laotree/uuid-compat-bench/internal/config"
	"github.com/Laotree/uuid-compat-bench/internal/uuid"
)

type Metric struct {
	Min     float64
	Median  float64
	Max     float64
	PerIter []float64
}

func (m *Metric) finalize() {
	if len(m.PerIter) == 0 {
		return
	}
	sorted := append([]float64(nil), m.PerIter...)
	sort.Float64s(sorted)
	m.Min = sorted[0]
	m.Median = sorted[len(sorted)/2]
	m.Max = sorted[len(sorted)-1]
}

type Latency struct {
	P50 float64
	P95 float64
	P99 float64
}

type PerfAtConcurrency struct {
	Concurrency        int
	InsertRowsPerSec   Metric
	ReadRowsPerSec     Metric
	EndToEndRowsPerSec Metric
	InsertLatency      Latency
	ReadLatency        Latency
	ReadMode           string
	insertLatAll       []float64
	readLatAll         []float64
}

type ScenarioPerf struct {
	Name               string
	Producer           string
	Consumer           string
	ConcurrencyResults []PerfAtConcurrency
}

func (s *ScenarioPerf) MaxRowsPerSec() float64 {
	max := 0.0
	for _, r := range s.ConcurrencyResults {
		if r.InsertRowsPerSec.Median > max {
			max = r.InsertRowsPerSec.Median
		}
	}
	return max
}

type PerfResult struct {
	Rows            int
	Concurrency     []int
	Iterations      int
	Warmup          time.Duration
	Duration        time.Duration
	WarnRegression  float64
	FailRegression  float64
	Scenarios       []*ScenarioPerf
	tableFor        map[string]string
	Version         string
	SaturationNotes []string
}

func (r *PerfResult) baseline() *ScenarioPerf { return r.Scenario("google -> google") }
func (r *PerfResult) candidate() *ScenarioPerf {
	return r.Scenario("stdlib -> stdlib")
}

func (r *PerfResult) RegressionPercent() float64 {
	b := r.baseline()
	c := r.candidate()
	if b == nil || c == nil {
		return 0
	}
	base := b.MaxRowsPerSec()
	cand := c.MaxRowsPerSec()
	if base <= 0 {
		return 0
	}
	return (base - cand) / base * 100
}

func (r *PerfResult) Verdict() string {
	compat := true
	for _, s := range r.Scenarios {
		for _, pac := range s.ConcurrencyResults {
			if pac.InsertRowsPerSec.Median <= 0 {
				compat = false
			}
		}
	}
	if !compat {
		return "FAIL"
	}
	reg := r.RegressionPercent()
	switch {
	case reg > r.FailRegression:
		return "FAIL"
	case reg > r.WarnRegression:
		return "WARNING"
	default:
		return "PASS"
	}
}

func (r *PerfResult) AllPass() bool {
	return r.Verdict() == "PASS" || r.Verdict() == "WARNING"
}

func (r *PerfResult) TableFor(name string) string { return r.tableFor[name] }

func (r *PerfResult) Scenario(name string) *ScenarioPerf {
	for _, s := range r.Scenarios {
		if s.Name == name {
			return s
		}
	}
	return nil
}

func (r *PerfResult) MaxRowsPerSec(name string) float64 {
	sc := r.Scenario(name)
	if sc == nil {
		return 0
	}
	return sc.MaxRowsPerSec()
}

type scenarioReq struct {
	producer uuid.Provider
	consumer uuid.Provider
	name     string
	table    string
	version  string
}

type passStats struct {
	insertRows    int64
	insertBatches int64
	insertWall    time.Duration
	readRows      int64
	readQueries   int64
	readWall      time.Duration
	insertLatUs   []float64
	readLatUs     []float64
}

func RunBenchmark(ctx context.Context, client *clickhouse.Client, cfg config.Config) (*PerfResult, error) {
	res := &PerfResult{
		Rows:           cfg.Rows,
		Concurrency:    cfg.Concurrency,
		Iterations:     cfg.Iterations,
		Warmup:         cfg.Warmup,
		Duration:       cfg.Duration,
		WarnRegression: cfg.WarnRegression,
		FailRegression: cfg.FailRegression,
		tableFor:       map[string]string{},
		Version:        client.Version(),
	}
	for _, pair := range uuid.Pairs() {
		name := uuid.PairName(pair[0], pair[1])
		table := cfg.Table + "_bench_" + shortName(name)
		res.tableFor[name] = table
		if err := client.DropTable(ctx, table); err != nil {
			return nil, fmt.Errorf("drop table %s: %w", table, err)
		}
		if err := client.EnsureSchema(ctx, table); err != nil {
			return nil, fmt.Errorf("create table %s: %w", table, err)
		}
		defer client.DropTable(ctx, table)
	}

	for _, pair := range uuid.Pairs() {
		name := uuid.PairName(pair[0], pair[1])
		sp := &ScenarioPerf{Name: name, Producer: pair[0].Name(), Consumer: pair[1].Name()}
		req := scenarioReq{pair[0], pair[1], name, res.tableFor[name], cfg.UUIDVersion}
		for _, c := range cfg.Concurrency {
			log.Printf("benchmark: %s concurrency=%d", name, c)
			sp.ConcurrencyResults = append(sp.ConcurrencyResults, benchmarkAtConcurrency(ctx, client, req, c, cfg))
		}
		res.Scenarios = append(res.Scenarios, sp)
	}
	res.detectSaturation()
	return res, nil
}

func benchmarkAtConcurrency(ctx context.Context, client *clickhouse.Client, req scenarioReq, concurrency int, cfg config.Config) PerfAtConcurrency {
	pac := PerfAtConcurrency{Concurrency: concurrency}
	pac.InsertRowsPerSec.PerIter = make([]float64, 0, cfg.Iterations)
	pac.ReadRowsPerSec.PerIter = make([]float64, 0, cfg.Iterations)
	pac.EndToEndRowsPerSec.PerIter = make([]float64, 0, cfg.Iterations)

	warmupPass(ctx, client, req, concurrency, cfg.Warmup)
	readMode, nativeOK := probeReadMode(ctx, client, req, concurrency)
	pac.ReadMode = readMode

	for i := 0; i < cfg.Iterations; i++ {
		st := runIteration(ctx, client, req, concurrency, cfg.Duration, nativeOK)
		pac.InsertRowsPerSec.PerIter = append(pac.InsertRowsPerSec.PerIter, rowsPerSec(st.insertRows, st.insertWall))
		pac.ReadRowsPerSec.PerIter = append(pac.ReadRowsPerSec.PerIter, rowsPerSec(st.readRows, st.readWall))
		pac.EndToEndRowsPerSec.PerIter = append(pac.EndToEndRowsPerSec.PerIter, rowsPerSec(st.insertRows, st.insertWall+st.readWall))
		pac.insertLatAll = append(pac.insertLatAll, st.insertLatUs...)
		pac.readLatAll = append(pac.readLatAll, st.readLatUs...)
	}
	pac.InsertRowsPerSec.finalize()
	pac.ReadRowsPerSec.finalize()
	pac.EndToEndRowsPerSec.finalize()
	pac.InsertLatency = latencyFrom(pac.insertLatAll)
	pac.ReadLatency = latencyFrom(pac.readLatAll)
	return pac
}

func latencyFrom(lats []float64) Latency {
	if len(lats) == 0 {
		return Latency{}
	}
	sort.Float64s(lats)
	return Latency{
		P50: percentileSorted(lats, 0.50),
		P95: percentileSorted(lats, 0.95),
		P99: percentileSorted(lats, 0.99),
	}
}

func warmupPass(ctx context.Context, client *clickhouse.Client, req scenarioReq, concurrency int, d time.Duration) {
	if d <= 0 {
		return
	}
	runIteration(ctx, client, req, concurrency, d, false)
}

func probeReadMode(ctx context.Context, client *clickhouse.Client, req scenarioReq, concurrency int) (string, bool) {
	client.TruncateTable(ctx, req.table)
	st := runInsertPhase(ctx, client, req, concurrency, time.Second)
	if st.insertRows == 0 {
		return "unknown", false
	}
	rows, err := client.Query(ctx, fmt.Sprintf("SELECT id FROM %s LIMIT 1", req.table))
	if err != nil {
		return "unknown", false
	}
	defer rows.Close()
	if rows.Next() {
		var b [16]byte
		if err := rows.Scan(req.consumer.ScanValue(&b)); err == nil {
			return "native", true
		}
	}
	return "bridge", false
}

func runIteration(ctx context.Context, client *clickhouse.Client, req scenarioReq, concurrency int, d time.Duration, nativeOK bool) passStats {
	if err := client.TruncateTable(ctx, req.table); err != nil {
		log.Printf("benchmark: truncate %s: %v", req.table, err)
	}
	insert := runInsertPhase(ctx, client, req, concurrency, d)
	read := runReadPhase(ctx, client, req, concurrency, insert.insertRows, nativeOK)
	insert.readRows = read.readRows
	insert.readQueries = read.readQueries
	insert.readWall = read.readWall
	insert.readLatUs = read.readLatUs
	return insert
}

func runInsertPhase(ctx context.Context, client *clickhouse.Client, req scenarioReq, concurrency int, d time.Duration) passStats {
	start := time.Now()
	perWorker := make([]passStats, concurrency)
	wg := sync.WaitGroup{}
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			st := &perWorker[w]
			seq := int64(0)
			for time.Since(start) < d {
				batch, err := client.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s (seq, id)", req.table))
				if err != nil {
					break
				}
				batchStart := time.Now()
				var rows int64
				for i := 0; i < batchSize; i++ {
					seq++
					b := genUUID(req.producer, req.version)
					if err := batch.Append(seq, req.producer.InsertValue(b)); err != nil {
						batch.Abort()
						break
					}
					rows++
				}
				if rows == 0 {
					break
				}
				if err := batch.Send(); err != nil {
					break
				}
				st.insertRows += rows
				st.insertBatches++
				st.insertLatUs = append(st.insertLatUs, float64(time.Since(batchStart).Microseconds())/float64(rows))
			}
		}(w)
	}
	wg.Wait()
	var total passStats
	for _, st := range perWorker {
		total.insertRows += st.insertRows
		total.insertBatches += st.insertBatches
		total.insertLatUs = append(total.insertLatUs, st.insertLatUs...)
	}
	total.insertWall = time.Since(start)
	return total
}

func runReadPhase(ctx context.Context, client *clickhouse.Client, req scenarioReq, concurrency int, totalRows int64, nativeOK bool) passStats {
	start := time.Now()
	perWorker := make([]passStats, concurrency)
	next := atomic.Int64{}
	getChunk := func() (lo, hi int64, ok bool) {
		lo = next.Add(4096) - 4096
		if lo >= totalRows {
			return 0, 0, false
		}
		hi = lo + 4096
		if hi > totalRows {
			hi = totalRows
		}
		return lo, hi, true
	}
	wg := sync.WaitGroup{}
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			st := &perWorker[w]
			for {
				lo, hi, ok := getChunk()
				if !ok {
					break
				}
				queryStart := time.Now()
				rows, err := client.Query(ctx, fmt.Sprintf("SELECT id FROM %s WHERE seq >= ? AND seq < ? ORDER BY seq", req.table), lo, hi)
				if err != nil {
					break
				}
				var scanned int64
				for rows.Next() {
					if nativeOK {
						var b [16]byte
						if err := rows.Scan(req.consumer.ScanValue(&b)); err != nil {
							break
						}
					} else {
						var s string
						if err := rows.Scan(&s); err != nil {
							break
						}
					}
					scanned++
				}
				if err := rows.Err(); err != nil {
					rows.Close()
					break
				}
				rows.Close()
				if scanned == 0 {
					break
				}
				st.readRows += scanned
				st.readQueries++
				st.readLatUs = append(st.readLatUs, float64(time.Since(queryStart).Microseconds())/float64(scanned))
			}
		}(w)
	}
	wg.Wait()
	var total passStats
	for _, st := range perWorker {
		total.readRows += st.readRows
		total.readQueries += st.readQueries
		total.readLatUs = append(total.readLatUs, st.readLatUs...)
	}
	total.readWall = time.Since(start)
	return total
}

func rowsPerSec(rows int64, wall time.Duration) float64 {
	if wall <= 0 {
		return 0
	}
	return float64(rows) / wall.Seconds()
}

func percentileSorted(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func (r *PerfResult) detectSaturation() {
	cand := r.candidate()
	if cand == nil {
		return
	}
	max := 0.0
	for _, pac := range cand.ConcurrencyResults {
		if pac.InsertRowsPerSec.Median > max {
			max = pac.InsertRowsPerSec.Median
		}
	}
	for _, pac := range cand.ConcurrencyResults {
		if max > 0 && pac.InsertRowsPerSec.Median >= 0.95*max {
			r.SaturationNotes = append(r.SaturationNotes, fmt.Sprintf("throughput saturation detected around concurrency=%d (%.2fM rows/s)", pac.Concurrency, pac.InsertRowsPerSec.Median/1e6))
			return
		}
	}
}

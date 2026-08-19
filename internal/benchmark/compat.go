package benchmark

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Laotree/uuid-compat-bench/internal/clickhouse"
	"github.com/Laotree/uuid-compat-bench/internal/config"
	"github.com/Laotree/uuid-compat-bench/internal/uuid"
)

const batchSize = 32_768

type Counters struct {
	Generated    int64
	Inserted     int64
	Read         int64
	Matched      int64
	Mismatched   int64
	InsertErrors int64
	ReadErrors   int64
	DecodeErrors int64
}

func (c Counters) Pass() bool {
	return c.Mismatched == 0 && c.InsertErrors == 0 && c.ReadErrors == 0 && c.DecodeErrors == 0
}

func (c Counters) GenOK() bool {
	return c.Inserted == c.Generated && c.InsertErrors == 0
}

type ScenarioResult struct {
	Name     string
	Producer string
	Consumer string
	Native   Counters
	Bridge   Counters
	Note     string
}

func (s ScenarioResult) Pass() bool {
	return s.Native.Pass()
}

type CompatResult struct {
	Rows      int
	Version   string
	Scenarios []ScenarioResult
	tableFor  map[string]string
}

func (r *CompatResult) AllPass() bool {
	for _, s := range r.Scenarios {
		if !s.Pass() {
			return false
		}
	}
	return true
}

func (r *CompatResult) TableFor(name string) string {
	return r.tableFor[name]
}

func RunCompatibility(ctx context.Context, client *clickhouse.Client, cfg config.Config) (*CompatResult, error) {
	res := &CompatResult{Rows: cfg.Rows, Version: client.Version(), tableFor: map[string]string{}}
	for _, pair := range uuid.Pairs() {
		producer, consumer := pair[0], pair[1]
		name := uuid.PairName(producer, consumer)
		table := cfg.Table + "_" + shortName(name)
		res.tableFor[name] = table

		log.Printf("compatibility: %s (table %s)", name, table)
		if err := client.DropTable(ctx, table); err != nil {
			return nil, fmt.Errorf("drop table %s: %w", table, err)
		}
		if err := client.EnsureSchema(ctx, table); err != nil {
			return nil, fmt.Errorf("create table %s: %w", table, err)
		}
		defer client.DropTable(ctx, table)

		start := time.Now()
		sc := runScenario(ctx, client, table, producer, consumer, cfg.Rows, cfg.UUIDVersion)
		sc.Name = name
		sc.Producer = producer.Name()
		sc.Consumer = consumer.Name()
		sc.Note = "elapsed " + time.Since(start).Round(time.Millisecond).String()
		res.Scenarios = append(res.Scenarios, sc)
	}
	return res, nil
}

func genUUID(p uuid.Provider, version string) [16]byte {
	if version == "v7" {
		return p.NewV7()
	}
	return p.New()
}

func shortName(name string) string {
	switch name {
	case "google -> google":
		return "gg"
	case "stdlib -> stdlib":
		return "ss"
	case "google -> stdlib":
		return "gs"
	case "stdlib -> google":
		return "sg"
	}
	return "x"
}

func runScenario(ctx context.Context, client *clickhouse.Client, table string, producer, consumer uuid.Provider, rows int, version string) ScenarioResult {
	var sc ScenarioResult

	expected := insertRows(ctx, client, table, producer, rows, version, &sc.Native)
	readAndCompare(ctx, client, table, consumer, sc.Native.Inserted, expected, &sc.Native)
	bridgeRead(ctx, client, table, consumer, sc.Native.Inserted, expected, &sc.Bridge)
	return sc
}

func insertRows(ctx context.Context, client *clickhouse.Client, table string, producer uuid.Provider, rows int, version string, c *Counters) [][16]byte {
	expected := make([][16]byte, rows)
	seq := int64(0)
	for offset := 0; offset < rows; offset += batchSize {
		n := batchSize
		if offset+n > rows {
			n = rows - offset
		}
		batch, err := client.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s (seq, id)", table))
		if err != nil {
			c.InsertErrors += int64(n)
			return expected
		}
		failed := false
		for i := 0; i < n; i++ {
			b := genUUID(producer, version)
			c.Generated++
			expected[offset+i] = b
			if !failed {
				if err := batch.Append(seq, producer.InsertValue(b)); err != nil {
					c.InsertErrors++
					failed = true
					batch.Abort()
				}
			}
			seq++
		}
		if !failed {
			if err := batch.Send(); err != nil {
				c.InsertErrors += int64(n)
				continue
			}
			c.Inserted += int64(n)
		}
	}
	return expected
}

func readAndCompare(ctx context.Context, client *clickhouse.Client, table string, consumer uuid.Provider, rows int64, expected [][16]byte, c *Counters) {
	if rows == 0 {
		return
	}
	n := int(rows)
	queryRows, err := client.Query(ctx, fmt.Sprintf("SELECT id FROM %s ORDER BY seq", table))
	if err != nil {
		c.ReadErrors++
		return
	}
	defer queryRows.Close()
	for i := 0; queryRows.Next(); i++ {
		if i >= n {
			c.Mismatched++
			continue
		}
		var got [16]byte
		if err := queryRows.Scan(consumer.ScanValue(&got)); err != nil {
			c.DecodeErrors++
			continue
		}
		c.Read++
		if got == expected[i] {
			c.Matched++
		} else {
			c.Mismatched++
		}
	}
	if err := queryRows.Err(); err != nil {
		c.ReadErrors++
	}
}

func bridgeRead(ctx context.Context, client *clickhouse.Client, table string, consumer uuid.Provider, rows int64, expected [][16]byte, c *Counters) {
	if rows == 0 {
		return
	}
	n := int(rows)
	queryRows, err := client.Query(ctx, fmt.Sprintf("SELECT id FROM %s ORDER BY seq", table))
	if err != nil {
		c.ReadErrors++
		return
	}
	defer queryRows.Close()
	for i := 0; queryRows.Next(); i++ {
		if i >= n {
			c.Mismatched++
			continue
		}
		var s string
		if err := queryRows.Scan(&s); err != nil {
			c.ReadErrors++
			continue
		}
		b, err := consumer.Parse(s)
		if err != nil {
			c.DecodeErrors++
			continue
		}
		c.Read++
		if b == expected[i] {
			c.Matched++
		} else {
			c.Mismatched++
		}
	}
	if err := queryRows.Err(); err != nil {
		c.ReadErrors++
	}
}

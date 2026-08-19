package clickhouse

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/Laotree/uuid-compat-bench/internal/config"
)

type Client struct {
	conn driver.Conn
}

func Connect(ctx context.Context, cfg config.Config) (*Client, error) {
	opts := &clickhouse.Options{
		Addr: []string{net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{},
	}
	if cfg.Secure {
		opts.TLS = &tls.Config{}
	}
	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open clickhouse connection: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := conn.Ping(pingCtx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping clickhouse %s:%d: %w", cfg.Host, cfg.Port, err)
	}
	return &Client{conn: conn}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Version() string {
	if sv, err := c.conn.ServerVersion(); err == nil {
		return sv.String()
	}
	return "unknown"
}

func (c *Client) Exec(ctx context.Context, query string, args ...any) error {
	return c.conn.Exec(ctx, query, args...)
}

func (c *Client) Select(ctx context.Context, dest any, query string, args ...any) error {
	return c.conn.Select(ctx, dest, query, args...)
}

func (c *Client) PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error) {
	return c.conn.PrepareBatch(ctx, query, opts...)
}

func (c *Client) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	return c.conn.Query(ctx, query, args...)
}

package clickhouse

import (
	"context"
	"fmt"
)

func (c *Client) EnsureSchema(ctx context.Context, table string) error {
	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s
(
    seq UInt64,
    id UUID
)
ENGINE = MergeTree
ORDER BY seq`, table)
	return c.Exec(ctx, query)
}

func (c *Client) DropTable(ctx context.Context, table string) error {
	return c.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", table))
}

func (c *Client) TruncateTable(ctx context.Context, table string) error {
	return c.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE IF EXISTS %s", table))
}

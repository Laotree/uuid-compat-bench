CREATE TABLE IF NOT EXISTS uuid_compat_test
(
    seq UInt64,
    id UUID
)
ENGINE = MergeTree
ORDER BY seq;
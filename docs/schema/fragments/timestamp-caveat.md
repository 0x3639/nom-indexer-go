!!! note "Timestamp encoding"
    All timestamps are stored as **Unix seconds in `BIGINT`** (UTC). They are
    never `TIMESTAMP`/`TIMESTAMPTZ`. Convert in SQL with
    `to_timestamp(<col>) AT TIME ZONE 'UTC'` to avoid session-timezone
    surprises. Full discussion in `schema/conventions.md` under "Timestamps".

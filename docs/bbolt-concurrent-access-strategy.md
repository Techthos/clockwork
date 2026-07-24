# bbolt concurrent access strategy

How clockwork lets the TUI and the MCP server work on one database file at the same time.

## The constraint

bbolt takes its file lock at **`Open`, not per transaction**:

- a read-write open holds a process-wide **exclusive** lock until `Close`
- a `ReadOnly` open holds a **shared** lock, which still blocks every writer
- the lock is per open file description, so a *second handle in the same process* collides
  exactly like a second process would

The consequence: any design that keeps a handle open for the process lifetime makes the two
surfaces mutually exclusive. That was clockwork's old behavior — starting the TUI while the MCP
server ran failed with `failed to open database: timeout`, and vice versa.

## The model: connection per operation

`db.Store` holds the **path**, never a handle. Every operation opens bbolt, runs one short
transaction and closes again:

- `Store.view(fn)` — `ReadOnly: true` open, `db.View`, close
- `Store.update(fn)` — read-write open, `db.Update`, close

An idle process therefore holds **no lock at all**, which is the only way two processes can both
write. The cost is one open/mmap per operation; at single-user scale (a keypress, a tool call)
that is invisible.

### Bootstrap

A `ReadOnly` open cannot create a missing file, so `db.New` performs exactly one read-write open
at startup: it creates the file, runs the idempotent bucket migration
(`CreateBucketIfNotExists` for `projects` and `entries`) and closes.

### Retry with backoff

Two processes will occasionally want the lock at the same moment. `Store.open` uses a short
per-attempt `Timeout` (75ms) and retries on `errors.ErrTimeout` from `go.etcd.io/bbolt/errors`,
doubling the wait from 25ms to a 250ms ceiling, up to a 3s total budget. A brief collision becomes
a sub-second wait instead of a hard failure; a genuinely stuck lock still fails loudly, with the
path, the mode and the attempt count in the error.

Only a lock-acquire timeout is retried. Every other open error (missing directory, permissions,
corruption) fails immediately — retrying those just delays the report.

### Keep operations short

The lock is held for the whole `Open`→`Close` span, so nothing slow may happen inside `view`/
`update`: no git subprocesses, no network, no user I/O. Callers do that work first and pass the
result in. Each atomic use-case is a single `update` transaction.

### Change detection

`Store.TxID()` exposes bbolt's monotonic committed transaction id. It advances exactly when
someone commits a write, so a long-lived reader can poll it to notice another process's changes
without scanning any data. The TUI reloads its lists on navigation and does not poll today; `TxID`
is the hook if live refresh is added.

## What this does not give you

- **No cross-process transactions.** Two writers still serialize; the second waits for the first
  to close. Last commit wins on the same record — there is no optimistic-concurrency check on
  `updated_at`.
- **No lock-free reads during a long write.** A writer holding the exclusive lock blocks readers
  for the duration of its (short) operation.

## When this would be the wrong model

For a single-surface application, "open once, hold for the lifetime" is simpler and faster. The
per-operation model is specifically for the low-contention multi-process sharing clockwork needs.

# Infrastructure

This folder will hold local infrastructure and integration notes.

## Lapdog

Install command:

```sh
brew install datadog/lapdog/lapdog && lapdog reclaimo
```

If Homebrew cannot write to `/opt/homebrew/Cellar` or `/opt/homebrew/Library/Taps`, fix local Homebrew ownership before retrying:

```sh
sudo chown -R "$USER" /opt/homebrew/Cellar /opt/homebrew/Library/Taps
```

After installation, run the backend through Lapdog from `apps/api`:

```sh
lapdog go run .
```

Or start Lapdog in the background before running local commands:

```sh
lapdog start
lapdog status
```

Current local status after Phase 4:

```sh
[lapdog] Lapdog running at http://127.0.0.1:8126/info
```

Check the Datadog-compatible intake metadata directly:

```sh
curl http://127.0.0.1:8126/info
```

The web app proxies this endpoint at:

```text
http://localhost:3000/lapdog/info
```

Current Reclaimo UI usage:

- Shows whether the local Lapdog intake is reachable.
- Uses the backend event stream as the visible execution trace.
- Sends Go spans to Lapdog when the API starts with `DATADOG_ENABLED=true`.

Run traced backend:

```sh
cd apps/api
DATADOG_ENABLED=true DD_AGENT_ADDR=127.0.0.1:8126 go run .
```

Instrumented span names:

- `reclaimo.ingest_receipt`
- `reclaimo.price_check`
- `reclaimo.recovery_workflow`
- `reclaimo.publish_recovery_dossier`
- `reclaimo.trigger_payment_intent`

Verify traces are being received:

```sh
tail -80 /Users/work/.lapdog/lapdog.log
```

Look for `POST /v0.4/traces` and span trees containing the `reclaimo.*` span names above.

## ClickHouse

ClickHouse support will be added behind the event store interface. Local startup should continue to work without ClickHouse while credentials are being configured.

Current event table:

```sql
CREATE TABLE IF NOT EXISTS reclaimo.events (
  id String,
  type String,
  version String,
  timestamp DateTime64(3, 'UTC'),
  payload String
)
ENGINE = MergeTree
ORDER BY (timestamp, type, id);
```

Verification query:

```sh
curl --user 'default:<password>' \
  --data-binary 'SELECT type, count() FROM reclaimo.events GROUP BY type ORDER BY type' \
  https://c6yash1lix.us-east-1.aws.clickhouse.cloud:8443
```

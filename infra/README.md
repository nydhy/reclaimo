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

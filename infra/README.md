# Infrastructure

This folder will hold local infrastructure and integration notes.

## Lapdog

Requested command:

```sh
brew install datadog/lapdog/lapdog && lapdog reclaimo
```

The first install attempt failed because Homebrew could not write to `/opt/homebrew/Cellar` or `/opt/homebrew/Library/Taps`. Fix local Homebrew ownership before retrying:

```sh
sudo chown -R "$USER" /opt/homebrew/Cellar /opt/homebrew/Library/Taps
```

## ClickHouse

ClickHouse support will be added behind the event store interface. Local startup should continue to work without ClickHouse while credentials are being configured.


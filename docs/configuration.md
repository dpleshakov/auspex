# Configuration

Auspex is configured via a YAML file. By default it looks for `auspex.yaml` in the current working directory. Override the path with the `-config` flag:

```bash
./auspex -config /path/to/custom.yaml
```

A commented template with all available fields is included in every release archive as `auspex.example.yaml`.

## Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `port` | integer | `8080` | TCP port the HTTP server listens on |
| `db_path` | string | `auspex.db` | Path to the SQLite database file |
| `refresh_interval` | integer | `10` | Background sync interval, in minutes |
| `esi.client_id` | string | — | EVE SSO Client ID (required) |
| `esi.client_secret` | string | — | EVE SSO Client Secret (required) |
| `esi.callback_url` | string | — | OAuth2 callback URL (required); must match the EVE Developer App setting exactly |

## Example

```yaml
port: 8080
db_path: auspex.db
refresh_interval: 10

esi:
  client_id: "your-client-id"
  client_secret: "your-client-secret"
  callback_url: "http://localhost:8080/auth/eve/callback"
```

## Notes

**`auspex.yaml` must never be committed to version control** — it contains your EVE SSO credentials. The file is listed in `.gitignore` by default.

**`callback_url`** must match the Callback URL registered in your EVE Developer Application exactly, including the port. If you change `port`, update `callback_url` and your Developer App settings accordingly.

**`db_path`** can be an absolute path or relative to the working directory where Auspex is launched. The database file is created automatically on first run.

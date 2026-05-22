# NGINX Proxy UI

Web UI for managing NGINX TCP/UDP stream proxies. Parses and writes the `stream {}` block in your nginx.conf — no reverse proxy, no agent, no dependency.

![screenshot](https://img.shields.io/badge/nginx-tcp/udp-brightgreen)
![Go](https://img.shields.io/badge/Go-1.22-blue)
![License](https://img.shields.io/badge/license-MIT-green)

## Features

- **CRUD stream proxies** — listen address, protocol (TCP/UDP), multiple backends with weight/fail_timeout/backup
- **Toggle proxy options** — proxy_protocol, tcp_nodelay, so_keepalive, proxy_timeout, connect_timeout
- **Load balancing** — round-robin (default), least_conn, random, hash (with consistent toggle)
- **Extra directives** — freeform textarea for arbitrary nginx directives
- **SQLite auth** — single admin user, bcrypt passwords, forced password change on first login
- **Config validation** — runs `nginx -t` before reload, shows status in UI
- **Persistent ID tracking** — marks upstream/server blocks with ID comments so renames don't lose backends

## Quick Start

```bash
# download the binary for your platform
curl -LO https://github.com/b00p1/nginx-proxy-ui/releases/latest/download/nginx-proxy-ui-linux-amd64
chmod +x nginx-proxy-ui-linux-amd64

# run (defaults to :8742, /etc/nginx/nginx.conf, /etc/nginx-proxy-manager/auth.db)
sudo ./nginx-proxy-ui-linux-amd64
```

Open `http://your-host:8742` and log in with `admin` / `admin`. You'll be forced to change the password.

### Environment variables

| Variable | Default | Description |
|---|---|---|
| `LISTEN_ADDR` | `:8742` | Web UI listen address |
| `NGINX_CONF` | `/etc/nginx/nginx.conf` | Path to nginx configuration |
| `DB_PATH` | `/etc/nginx-proxy-manager/auth.db` | SQLite database location |

### Systemd service

The repo includes a [service unit](nginx-proxy-manager.service):

```bash
sudo cp nginx-proxy-manager.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now nginx-proxy-manager
```

## How it works

1. **Parser** — `findStreamBlock` locates the `stream { }` block in nginx.conf, `splitTopLevel` extracts individual upstream/server blocks
2. **ID comments** — `# nginx-proxy-manager-id: <uuid>` tags are written to each block so backends and options survive renames
3. **Builder** — regenerates upstream + server blocks from the proxy model, preserves everything outside `stream {}`
4. **Validation** — `nginx -t` is called before offering a reload in the UI

## Screenshots

![Login](/web/screenshots/login.png)
![Dashboard](/web/screenshots/dashboard.png)
![Edit Proxy](/web/screenshots/edit.png)

## Development

```bash
git clone https://github.com/b00p1/nginx-proxy-ui.git
cd nginx-proxy-ui
go build -o nginx-proxy-ui .

# test with a throwaway config
cat > /tmp/test.conf << 'EOF'
stream {
    upstream db {
        server 127.0.0.1:3306;
    }
    server {
        listen 3306;
        proxy_pass db;
    }
}
EOF
NGINX_CONF=/tmp/test.conf DB_PATH=/tmp/test.db ./nginx-proxy-ui
```

## Release

Tag a version and the CI builds all four binaries + uploads a release:

```bash
git tag v1.0.1 && git push origin v1.0.1
```

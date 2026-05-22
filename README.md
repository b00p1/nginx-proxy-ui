# NGINX Proxy UI

Web UI for managing NGINX TCP/UDP stream proxies. Manages a separate config file included from your main nginx.conf — never touches your existing config.

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
- **Zero config modification** — reads/writes a standalone file, your nginx.conf stays untouched

## Quick Start

### 1. Install NGINX stream module

```bash
# Debian / Ubuntu
sudo apt install libnginx-mod-stream
```

Verify the module loads:
```bash
echo 'load_module /usr/lib/nginx/modules/ngx_stream_module.so;' | sudo tee /etc/nginx/modules-enabled/50-mod-stream.conf
nginx -t
```

### 2. Add include to your nginx.conf

In `/etc/nginx/nginx.conf`, add a `stream` block at the top level:

```nginx
stream {
    include /etc/nginx/stream-manager/stream.conf;
}
```

Create the file so nginx accepts the config:
```bash
sudo mkdir -p /etc/nginx/stream-manager && sudo touch /etc/nginx/stream-manager/stream.conf
nginx -t   # should pass
```

### 3. Download and run

```bash
# download the binary for your platform
curl -LO https://github.com/b00p1/nginx-proxy-ui/releases/latest/download/nginx-proxy-ui-linux-amd64
chmod +x nginx-proxy-ui-linux-amd64

# run (defaults to :8742)
sudo ./nginx-proxy-ui-linux-amd64
```

Open `http://your-host:8742` and log in with `admin` / `admin`. You'll be forced to change the password.

### Environment variables

| Variable | Default | Description |
|---|---|---|
| `LISTEN_ADDR` | `:8742` | Web UI listen address |
| `STREAM_CONF` | `/etc/nginx/stream-manager/stream.conf` | Stream config file path |
| `DB_PATH` | `/etc/nginx-proxy-manager/auth.db` | SQLite database location |

### Systemd service

The repo includes a [service unit](nginx-proxy-manager.service):

```bash
sudo cp nginx-proxy-manager.service /etc/systemd/system/

# create required directories
sudo mkdir -p /etc/nginx/stream-manager /etc/nginx-proxy-manager

sudo systemctl daemon-reload
sudo systemctl enable --now nginx-proxy-manager
```

## How it works

1. **Standalone file** — the app manages `/etc/nginx/stream-manager/stream.conf` which contains bare upstream/server blocks (no `stream {}` wrapper)
2. **Include** — your main nginx.conf has `stream { include ...; }` which pulls in the managed content
3. **ID comments** — `# nginx-proxy-manager-id: <uuid>` tags track each proxy across renames
4. **Validation** — `nginx -t` is called before offering a reload in the UI
5. **Your config is safe** — the app never reads or modifies your main nginx.conf

## Development

```bash
git clone https://github.com/b00p1/nginx-proxy-ui.git
cd nginx-proxy-ui
go build -o nginx-proxy-ui .

# test with a throwaway config
mkdir -p /tmp/nginx-proxy-manager
STREAM_CONF=/tmp/nginx-proxy-manager/stream.conf DB_PATH=/tmp/nginx-proxy-manager/auth.db ./nginx-proxy-ui
```

## Release

Tag a version and the CI builds all four binaries + uploads a release:

```bash
git tag v1.0.1 && git push origin v1.0.1
```

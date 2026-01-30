# VS Code Web Integration for States Browser

This document describes how to integrate a web-based VS Code (code-server or openvscode-server) with the States Browser feature, allowing developers to open state data directly in VS Code from the browser.

## Overview

The goal is to provide an "Open in VS Code" button in the States Browser that:
1. Writes the state data to a temp file on the server
2. Opens a web-based VS Code instance with that file loaded

This works because both stratasave and the VS Code server run on the same machine, sharing the filesystem.

## Options

### code-server (Recommended)

[code-server](https://github.com/coder/code-server) by Coder is VS Code running in the browser. It's a single binary, easy to set up, and widely used.

**Pros:**
- Simple installation (single binary or Docker)
- Built-in password authentication
- Active community and good documentation
- Can run alongside stratasave on the same server

**Installation:**
```bash
# Using the install script
curl -fsSL https://code-server.dev/install.sh | sh

# Or via Docker
docker run -it -p 8080:8080 -v "$PWD:/home/coder/project" codercom/code-server
```

**Configuration (~/.config/code-server/config.yaml):**
```yaml
bind-addr: 127.0.0.1:8080
auth: password
password: your-secure-password
cert: false
```

### openvscode-server

[openvscode-server](https://github.com/gitpod-io/openvscode-server) is Microsoft's official open-source VS Code for the web, maintained by Gitpod.

**Pros:**
- Official Microsoft build
- Closer to standard VS Code experience
- Good for those who want maximum compatibility

**Installation:**
```bash
# Download from releases
wget https://github.com/gitpod-io/openvscode-server/releases/download/openvscode-server-v1.85.1/openvscode-server-v1.85.1-linux-x64.tar.gz
tar -xzf openvscode-server-v1.85.1-linux-x64.tar.gz
cd openvscode-server-v1.85.1-linux-x64
./bin/openvscode-server --port 8080
```

## Deep-Link URL Format

### code-server
```
https://code.example.com/?folder=/tmp&file=/tmp/stratasave-state-game-user-id.json
```

Or to open just the file:
```
https://code.example.com/?open=/tmp/stratasave-state-game-user-id.json
```

### openvscode-server
```
https://code.example.com/?folder=/tmp&openFile=/tmp/stratasave-state-game-user-id.json
```

## Stratasave Implementation

### 1. Configuration

Add a new environment variable for the code-server URL:

```go
// In bootstrap/config.go or appconfig.go
CodeServerURL string `env:"STRATASAVE_CODE_SERVER_URL"` // e.g., "https://code.example.com"
```

### 2. Handler (savebrowser/handler.go)

Restore and modify the VS Code handler:

```go
// HandleOpenInVSCode writes state to temp file and redirects to code-server
func (h *Handler) HandleOpenInVSCode(w http.ResponseWriter, r *http.Request) {
    // Return 404 if code-server URL not configured
    if h.codeServerURL == "" {
        http.Error(w, "VS Code integration not configured", http.StatusNotFound)
        return
    }

    ctx, cancel := context.WithTimeout(r.Context(), timeouts.Short())
    defer cancel()

    game := chi.URLParam(r, "game")
    idStr := chi.URLParam(r, "id")

    id, err := primitive.ObjectIDFromHex(idStr)
    if err != nil {
        http.Error(w, "Invalid save ID", http.StatusBadRequest)
        return
    }

    save, err := h.store.GetSave(ctx, game, id)
    if err != nil {
        http.Error(w, "Failed to get save", http.StatusInternalServerError)
        return
    }

    // Marshal and write to temp file
    jsonData, _ := json.MarshalIndent(save.SaveData, "", "  ")
    filename := fmt.Sprintf("stratasave-state-%s-%s-%s.json", game, save.UserID, idStr)
    filePath := filepath.Join(os.TempDir(), filename)
    os.WriteFile(filePath, jsonData, 0644)

    // Redirect to code-server with the file
    redirectURL := fmt.Sprintf("%s/?open=%s", h.codeServerURL, url.QueryEscape(filePath))
    http.Redirect(w, r, redirectURL, http.StatusFound)
}
```

### 3. Routes (savebrowser/routes.go)

```go
r.Get("/{game}/{id}/vscode", h.HandleOpenInVSCode)
```

### 4. Template (savebrowser_list.gohtml)

Only show the VS Code button if configured:

```html
{{ if .CodeServerURL }}
<a href="/console/api/state/{{ $.SelectedGame }}/{{ $save.ID }}/vscode"
   target="_blank"
   class="hover:opacity-80 transition-opacity"
   title="Open in VS Code">
  <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none">
    <path d="M17.5 3L7 12L17.5 21V15.5L21 17V7L17.5 8.5V3Z" fill="#007ACC"/>
    <path d="M7 12L2 8V16L7 12Z" fill="#1F9CF0"/>
    <path d="M17.5 3L7 12L2 8L17.5 3Z" fill="#0065A9"/>
    <path d="M17.5 21L7 12L2 16L17.5 21Z" fill="#0065A9"/>
  </svg>
</a>
{{ end }}
```

## Deployment Architecture

### Option A: Same Server (Simple)

```
┌─────────────────────────────────────────┐
│              AWS EC2 Instance           │
│  ┌─────────────────┐  ┌──────────────┐  │
│  │   stratasave    │  │  code-server │  │
│  │   (port 443)    │  │  (port 8080) │  │
│  └────────┬────────┘  └──────┬───────┘  │
│           │                  │          │
│           └──────┬───────────┘          │
│                  │                      │
│           ┌──────┴───────┐              │
│           │  /tmp files  │              │
│           └──────────────┘              │
└─────────────────────────────────────────┘
```

Run code-server on a different port, optionally behind nginx:

```nginx
# /etc/nginx/sites-available/code-server
server {
    listen 443 ssl;
    server_name code.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

### Option B: Subdomain/Path on Same Domain

If stratasave is at `app.example.com`, run code-server at `app.example.com/code/`:

```nginx
location /code/ {
    proxy_pass http://127.0.0.1:8080/;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    # ... other headers
}
```

## Security Considerations

### Authentication

1. **code-server built-in auth**: Simple password protection
   ```yaml
   auth: password
   password: your-secure-password
   ```

2. **Proxy auth**: Use nginx to require the same authentication as stratasave
   ```nginx
   location / {
       auth_request /auth;
       proxy_pass http://127.0.0.1:8080;
   }
   ```

3. **IP restriction**: Limit access to specific IPs or VPN
   ```nginx
   allow 10.0.0.0/8;
   deny all;
   ```

### File Access

code-server runs with the permissions of its user. Consider:
- Running it as a dedicated user with limited permissions
- Restricting which directories it can access
- Using a chroot or container

### Temp File Cleanup

Add a cron job to clean up old state files:
```bash
# Clean up stratasave temp files older than 1 hour
0 * * * * find /tmp -name 'stratasave-state-*.json' -mmin +60 -delete
```

## Systemd Service

Create `/etc/systemd/system/code-server.service`:

```ini
[Unit]
Description=code-server
After=network.target

[Service]
Type=simple
User=stratasave
ExecStart=/usr/bin/code-server --bind-addr 127.0.0.1:8080
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable code-server
sudo systemctl start code-server
```

## Testing

1. Install code-server on your server
2. Access it directly at `http://your-server:8080` to verify it works
3. Set `STRATASAVE_CODE_SERVER_URL=http://your-server:8080` (or https with proper setup)
4. Restart stratasave
5. The VS Code button should appear in States Browser
6. Clicking it should open code-server with the state file loaded

## Future Enhancements

- **Read-only mode**: Open files in read-only mode to prevent accidental edits
- **Workspace files**: Create a workspace with multiple state files
- **Direct editing**: Allow saving changes back to the database (requires careful security review)

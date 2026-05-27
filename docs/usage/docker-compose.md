# Docker Compose

Tusk mendukung Docker Compose untuk multi-container applications.

## File Format

Gunakan `docker-compose.yml` atau `tusk-compose.yaml`:

```yaml
version: "3.8"

services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
    depends_on:
      - db
    volumes:
      - ./html:/var/www/html
    environment:
      - NODE_ENV=production

  db:
    image: postgres:15
    volumes:
      - db-data:/var/lib/postgresql/data
    environment:
      - POSTGRES_PASSWORD=secret

volumes:
  db-data:

networks:
  default:
    driver: bridge
```

## Commands

### `tusk compose up`
Create dan start semua services.

```bash
tusk compose up              # Foreground
tusk compose up -d           # Detached
tusk compose up --build       # Build images first
```

### `tusk compose down`
Stop dan remove services, networks.

```bash
tusk compose down            # Stop containers
tusk compose down -v         # Also remove volumes
```

### `tusk compose ps`
List running services.

```bash
tusk compose ps
# Output:
# NAME      IMAGE      STATUS
# web       nginx      running
# db        postgres   running
```

### `tusk compose logs`
View logs dari semua services.

```bash
tusk compose logs            # All services
tusk compose logs web        # Specific service
tusk compose logs -f         # Follow logs
```

### `tusk compose build`
Build images (untuk Dockerfile).

```bash
tusk compose build
tusk compose build --no-cache
```

### `tusk compose stop`
Stop services tanpa remove.

```bash
tusk compose stop
```

### `tusk compose rm`
Remove stopped containers.

```bash
tusk compose rm
tusk compose rm -s           # Stop before removing
```

## Supported Compose Keys

| Key | Status | Description |
|-----|--------|-------------|
| `services` | ✅ | Service definitions |
| `image` | ✅ | Container image |
| `command` | ✅ | Override default command |
| `depends_on` | ✅ | Service dependencies |
| `ports` | ✅ | Port mappings |
| `volumes` | ✅ | Volume mounts |
| `environment` | ✅ | Environment variables |
| `networks` | ✅ | Network configuration |
| `build` | ⬜ | Dockerfile build (planned) |
| `restart` | ⬜ | Restart policy (planned) |
| `labels` | ✅ | Container labels |
| `healthcheck` | ⬜ | Health check (planned) |

## Dependency Resolution

Services di-start berdasarkan `depends_on`:

```
web (depends on: db) ──────► db
frontend (depends on: web) ─┘
```

Services di-stop dalam urutan terbalik:
1. frontend
2. web
3. db

## Environment Variables

```yaml
services:
  app:
    environment:
      - DEBUG=1
      - DATABASE_URL=postgres://...
    env_file:
      - .env
```

## Volume Types

### Bind Mount

```yaml
volumes:
  - ./data:/app/data        # Host path to container path
  - /tmp/cache:/tmp/cache    # Read-only
```

### Named Volume

```yaml
volumes:
  db-data:                  # Named volume

# Or with driver:
volumes:
  db-data:
    driver: local
```

---

*Back to [docs](../README.md)*
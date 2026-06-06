# Docker Compose

Tusk supports Docker Compose for multi-container applications.

## File Format

Use `docker-compose.yml` or `tusk-compose.yaml`:

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
Create and start all services.

```bash
tusk compose up              # Foreground
tusk compose up -d           # Detached
tusk compose up --build       # Build images first
```

### `tusk compose down`
Stop and remove services, networks.

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
View logs from all services.

```bash
tusk compose logs            # All services
tusk compose logs web        # Specific service
tusk compose logs -f         # Follow logs
```

### `tusk compose build`
Build images (for Dockerfile).

```bash
tusk compose build
tusk compose build --no-cache
```

### `tusk compose stop`
Stop services without removing.

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
| `services` | partial | Service definitions |
| `image` | partial | Container image |
| `command` | partial | Override default command |
| `depends_on` | partial | Service dependencies |
| `ports` | partial | Port mappings |
| `volumes` | partial | Volume mounts |
| `environment` | partial | Environment variables |
| `networks` | partial | Network configuration |
| `build` | stub | Dockerfile build |
| `restart` | stub | Restart policy |
| `labels` | partial | Container labels |
| `healthcheck` | stub | Health check |

## Dependency Resolution

Services are started based on `depends_on`:

```
web (depends on: db) ──────► db
frontend (depends on: web) ─┘
```

Services are stopped in reverse order:
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

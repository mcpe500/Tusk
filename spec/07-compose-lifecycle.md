# Compose Lifecycle Specification

## Scope

- `tusk compose up` and compose parser.
- Other compose subcommands (`down`, `ps`, `build`, `logs`, `rm`, `stop`).

## Status

| Command | Status | Implementation |
|---|---|---|
| `tusk compose up` | partial | Parse file, compute dependency, start service through daemon |
| `tusk compose down` | stub | print placeholder |
| `tusk compose ps` | stub | print placeholder |
| `tusk compose build` | stub | print placeholder |
| `tusk compose logs` | stub | print placeholder |
| `tusk compose rm` | stub | print placeholder |
| `tusk compose stop` | stub | print placeholder |

## Compose Up Flow

1. `runCompose` parses `-f/--file` flag + subcommand.
2. `compose.Parser.Parse` reads YAML.
3. `Orchestrator.Up()`:
   - create networks,
   - create volumes,
   - order services via `resolveServiceOrder`,
   - for each service:
     - parse command,
     - `ContainerCreate`,
     - `ContainerStart`.

## Shortcomings

- `volumes` and `networks` are only string print.
- `depends_on` only validates order; no health check dependency wait.
- Service `build` path only rejects with text error.
- `service stop/remove/logs` in orchestrator is mostly placeholder.

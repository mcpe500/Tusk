# Tusk - Overview

## Apa itu Tusk?

**Tusk** adalah container runtime untuk Termux yang memanfaatkan QEMU VM sebagai pengganti Docker. Dengan Tusk, kamu bisa menjalankan container images dari Docker Hub di lingkungan Termux/Android.

> "Docker tidak bisa jalan di Termux? Oke, kita bikin sendiri."

## Kenapa Tusk Ada?

Docker adalah standar industri untuk containerization, tapi Docker butuh:
- `dockerd` (Linux daemon)
- Linux namespaces (pid, network, mount, dll)
- Cgroups untuk resource limiting
- Overlay filesystem

**Semuanya tidak tersedia di Termux/Android.**

QEMU adalah alternatif yang bisa berjalan di Termux. Dengan Alpine Linux (sangat ringan, ~50MB RAM), kita bisa membuat VM yang bertindak sebagai "container host".

## Arsitektur

```
┌─────────────────────────────────────────────────────────────┐
│                      Termux (Host)                          │
│                                                             │
│  ┌─────────┐    ┌──────────────────────────────────────┐  │
│  │  tusk   │───►│  QEMU VM (Alpine x86_64, TCG)          │  │
│  │   CLI   │    │                                        │  │
│  └─────────┘    │  ┌─────────────────────────────────┐   │  │
│     │           │  │         tuskd (daemon)           │   │  │
│     │           │  │  - Container management           │   │  │
│     │           │  │  - Image extraction              │   │  │
│     │           │  │  - Network (Dnsmasq)             │   │  │
│     │           │  └─────────────────────────────────┘   │  │
│     │           └──────────────────────────────────────────┘  │
│     │                        ▲                              │
│     │  9p mount              │                              │
│     │  ~/.tusk/              │ virtio-serial                │
│     ▼  (shared storage)       │ (.tusk/vms/serial.sock)     │
│  ~/.tusk/                                                   │
└─────────────────────────────────────────────────────────────┘
```

### Komponen Utama

| Komponen | Fungsi |
|----------|--------|
| `tusk` CLI | Command-line interface di host |
| `tuskd` | Daemon yang jalan di dalam VM |
| QEMU VM | Isolasi via virtualisasi |
| Image Store | Simpan dan pull OCI images |

## Perbandingan dengan Docker

| Aspek | Docker | Tusk |
|-------|--------|------|
| Isolation | Linux namespaces | QEMU VM |
| Startup Time | ~100ms | ~3-5 detik |
| Memory Overhead | ~10MB | ~50MB |
| Resource Usage | Low | Medium |
| Portability | Linux only | Any platform dengan QEMU |
| OCI Compatible | Yes | Partial |

## Fitur

- ✅ Pull images dari Docker Hub
- ✅ Run containers
- ✅ Docker Compose support
- ✅ OCI-compliant image format
- ⬜ Port forwarding (coming soon)
- ⬜ Volume mounts (coming soon)
- ⬜ Container exec (coming soon)

## License

MIT

---

*Back to [docs](./README.md)*
# Tusk Handoff — 2026-06-11

Docker-for-Termux/Android container runtime (Go). Goal: jalankan container/`docker compose` di HP tanpa root. **Arsitektur final: HYBRID — proot dulu (native arm64, cepat), QEMU+Docker-asli fallback. TIDAK ADA simulation mode.**

## TL;DR status
- Native proot runtime SUDAH nyata jalan (root cause exec ketemu, lihat bawah).
- `tusk pull` SUDAH fix (401 + multi-arch arm64).
- Daemon + CLI build OK, container betulan exec via proot.
- **Belum lulus acceptance:** `tusk compose up -d` masih bocor (ports/volumes di-drop, build fake, 4 subcommand stub), dan `install.sh` belum install proot + masih nyebut "simulation"/arahin ke VM 404.

## Root cause kunci (proot exec) — JANGAN LUPA
Termux inject `LD_PRELOAD=libtermux-exec-ld-preload.so` ke semua proses. Itu bikin `execve` proot ke rootfs asing gagal `ENOSYS` (pesan misleading "Function not implemented"). Fix terverifikasi:
```
env -u LD_PRELOAD PROOT_TMP_DIR="$HOME/.tusk/tmp" \
  proot --kill-on-exit --sysvipc --link2symlink \
        --rootfs="$ROOTFS" -b /dev -b /proc -b /sys --cwd=/ \
        /bin/sh -c '<cmd>'
```
- WAJIB `env -u LD_PRELOAD`. WAJIB `PROOT_TMP_DIR` (Termux `/tmp` read-only).
- Same-arch (arm64 image di aarch64) = NO qemu, native cepat.
- Cross-arch = tambah `-q qemu-x86_64` (butuh `pkg install qemu-user-x86-64`).
- Image OS selalu "linux"; di Termux `runtime.GOOS=="android"` — jangan filter pakai GOOS, cocokin "linux" + GOARCH.

## Sudah dikerjakan (file)
- `internal/image/store.go` — Puller fix: prefix `library/` untuk official image, token scope cocok dgn manifest path, handle multi-arch manifest list (pilih arm64 digest, fallback amd64). Tambah field `Config.Entrypoint` + `Config.WorkingDir`.
- `internal/container/proot.go` (BARU) — ProotConfig, BuildProotArgs, prootEnviron (strip LD_PRELOAD), BuildCmd, qemuForArch, injectGuestEnv. Backend proot lengkap.
- `cmd/tuskd/runtime.go` (BARU) — Runtime real: Start (proot exec + PID asli + log file), Stop (kill process group), Remove, Logs, IsRunning. Ganti fake `Pid=12345`.
- `cmd/tuskd/runtime_image.go` (BARU) — Pull, ImageList (pakai repositories.json index), Exec.
- `cmd/tuskd/store.go` — meta pindah ke `<base>/<id>/meta.json`, rootfs `<base>/<id>/rootfs`, ID unik (timestamp+random), timestamp real, FindByNameOrID.
- `cmd/tuskd/rpc.go` — handler pakai Runtime nyata. Fix STATUS kosong (map `state`→wire `status`). parsePortMappings.
- `cmd/tuskd/simulation.go` — DIGANTI jadi runHostSocket/runHostMode (daemon proot native, listen `~/.tusk/vm/serial.sock`).
- `cmd/tuskd/main.go` — hapus jalur simulation; host mode = proot daemon nyata.
- `cmd/tuskd/daemon.go` — thread Runtime ke handleStream/handleRPC.

Build: `cd ~/Tusk && go build ./...` = OK. Binaries: `~/tusk`, `~/tuskd`.

## BUG / GAP yang HARUS dibereskan (prioritas)

### A. Runtime bugs (dari review + analisa sendiri)
1. **`runtime_image.go` Exec() panggil `r.imageConfig("")` (ref KOSONG)** — gak akan pernah resolve, env/arch selalu nil. Fix: Exec harus terima image ref container (lookup dari store/meta).
2. **PID tracking in-memory** (`r.running` map) — daemon restart = kehilangan handle proses, Stop/IsRunning buta. Fix: persist PID di meta.json, cek `/proc/<pid>` saat IsRunning.
3. **`effectiveCommand` abaikan Entrypoint** — cuma pakai `Config.Cmd`. Image dgn Entrypoint gagal jalan benar. Fix: gabung Entrypoint+Cmd.
4. **Race log file** — goroutine Wait() close file vs Logs() baca. Fix: jangan close, atau guard.
5. Verifikasi `injectGuestEnv` posisi `/usr/bin/env` benar untuk kasus same-arch DAN cross-arch (-q). Belum dites cross-arch.
6. Verifikasi path rootfs antara `store.go` (`<base>/<id>/rootfs`) dan `internal/container/runtime.go` PrepareRootfs SAMA — kalau beda, Start gak nemu rootfs. **CRITICAL, belum dikonfirmasi** (subagent #3 keburu interrupted).

### B. Compose (`tusk compose up -d` = acceptance test utama)
File: `cmd/tusk/compose.go`, `internal/compose/parser.go`.
1. `build`/`logs`/`rm`/`stop` = STUB print doang (compose.go:57-64). Wire ke RPC.
2. `-d`/`--detach` TIDAK diparse (compose.go:30-42); flag setelah `up` diabaikan diam-diam.
3. **Ports gak pernah dikirim** ke ContainerCreate (parser.go:160-169, svc.Ports diabaikan; ContainerCreateParams gak punya field port).
4. **Volumes gak dimapping** ke Mounts (parser.go:160-169).
5. Cuma network pertama dipakai (parser.go:167-169).
6. createNetwork FAKE print (parser.go:102-105) — padahal NetworkCreate RPC ada.
7. createVolume FAKE print (parser.go:111-114) — perlu VolumeCreate RPC.
8. `build` fabrikasi nama image `<proj>-<svc>-built` (parser.go:132-135) → ContainerCreate pasti gagal. Implement build asli atau fail jelas.
9. `down` stopService fake (parser.go:274-277); teardown asli di compose.go:156-174 TAPI hapus SEMUA container (gak project-scoped) — BAHAYA hapus container lain.
10. `compose ps` list semua, gak project-scoped (compose.go:195).
11. depends_on cuma ordering, gak tunggu readiness (boleh, low prio).

OK (gak perlu fix): RPC method names cocok daemon; `up` create+start beneran; env dikirim (termasuk env_file); socket path konsisten `~/.tusk/vm/serial.sock`.

### C. install.sh (one-liner acceptance test)
File: `scripts/install.sh` (+ `prebuilt-install.sh`, `auto-install.sh`, `cmd/tusk/system.go`).
1. **proot TIDAK diinstall** (install.sh:26 DEPS gak ada proot) — runtime gak bisa jalan. Fix: tambah `proot` (+ `qemu-user-x86-64` cross-arch), buang qemu-system/expect/socat yg gak dipakai native.
2. Banner sukses nyebut "simulation mode" + arahin `tusk install` (QEMU VM) (install.sh:74,100,105-106). Fix: rewrite jadi "native proot runtime", hapus arahan VM.
3. Komentar stale "Building tuskd-local for simulation mode" (install.sh:48-49). Rename `tuskd-local`→`tuskd`.
4. Daemon distart bare background (install.sh:75), mati saat shell close. Fix: nohup/disown + Termux:Boot/login hook.
5. Verifikasi cuma Ping (install.sh:94), gak smoke-test container. Fix: tambah `tusk run alpine echo ok` biar gagal kalau proot absent.
6. `cmd/tusk/system.go:188-213,240-305` — `tusk start` masih fallback simulation, `tusk install` masih trigger VM/QEMU. Fix: route native aarch64 ke proot daemon, matikan simulation fallback.
7. `prebuilt-install.sh:27,90,98` — URL qcow2 v0.1.0 hardcoded kemungkinan 404; 404 HTML lolos size-check → gunzip fail. Deprecate utk Termux atau validasi HTTP status+Content-Type.

## Test berikutnya (acceptance)
1. `cd ~/Tusk && go build -o ~/tusk ./cmd/tusk && go build -o ~/tuskd ./cmd/tuskd`
2. `pkill -f tuskd; rm -f ~/.tusk/vm/serial.sock; ~/tuskd &` (background)
3. `~/tusk pull alpine` → harus sukses (arm64).
4. `~/tusk run alpine echo hello` → harus print "hello" via proot.
5. Bikin `docker-compose.yml` 2-service, `~/tusk compose up -d` → container betulan jalan, ps tampil status.
Catatan build: `make` kadang gak recompile kalau binary udah ada — pakai `go build` langsung atau `rm` dulu.

## Env
proot + proot-distro terinstall, aarch64, net OK, `/dev/kvm` diblok (QEMU=TCG lambat → makanya proot diutamakan). proot-distro = reference impl (lihat ~line 1900: cuma tambah `-q emulator` kalau target_arch != device_arch).

## Belum dikonfirmasi (utang verifikasi)
- Path rootfs store vs PrepareRootfs (gap A6) — CEK DULU sebelum apa-apa.
- End-to-end pull→run→compose via daemon (test sebelumnya keburu interrupted).
- Cross-arch (-q qemu) belum pernah dites.

# DEFQON.1 Stream Recorder

A powerful terminal-based application for recording multiple Mixlr streams simultaneously with a beautiful TUI (Terminal User Interface). Built in Go as a single static binary, it watches every DEFQON.1 stage, records live DJ sets the moment they go online, and recovers automatically from stalled streams.

![Screenshot](screenshot_3.0.1.png)

<a href='https://ko-fi.com/revunix' target='_blank'><img height='36' style='border:0px;height:36px;' src='https://cdn.ko-fi.com/cdn/kofi1.png?v=3' border='0' alt='Buy Me a Coffee' /></a>

## ✨ Features

- 🎵 **Simultaneous recording** of all 14 Mixlr stages
- 🖥️ **Live TUI dashboard** with two tables, a log panel and a status bar
- 🟢 **Online & offline tracking** — every stage is always listed, with its current state
- 🎨 **Real stage colors** — each stage is rendered in its actual DEFQON.1 signature color
- 📊 **Real-time listener counts**, file sizes and set-end countdowns
- 📅 **Built-in timetable** with DJ set times, current-DJ detection and "starts in" countdowns
- 🔍 **Live detection** straight from the Mixlr `data.attributes.live` flag
- 🛡️ **Robust recovery** — stalled streams are detected and restarted automatically
- 🚀 **Cross-platform** — one static binary for macOS, Linux and Windows (no runtime needed)
- ⚡ **Graceful shutdown** — `q`, `Ctrl+C` and Docker `SIGTERM` all finish recordings cleanly

## 🖼️ The Interface

The dashboard is split into four regions:

```
┌─ Streams ──────────────────┐ ┌─ Timetable ───────────────┐
│ Stage Status Artist ...    │ │ Stage Time Artist Starts  │
│  RED  Rec.  Atmozfears ... │ │  UV  13:00 ...    2h 15m  │
│ BLUE  Off.       -     -   │ │ RED  14:00 ...    3h  0m  │
│  ...                       │ │  ...                      │
├─ Logs ───────────────────────────────────────────────────┤
│ 13:00:02 --- Checking channels at 13:00:02 ---           │
│ 13:00:03 [RED] Starting recording...                     │
├─ Status ─────────────────────────────────────────────────┤
│ Active: 3/14 | Total Listeners: 12.345                   │
└──────────────────────────────────────────────────────────┘
```

- **Streams** (left) lists every stage with `Stage | Status | Artist | Listeners | Size | Ends in`.
  Status colors: <span style="color:#00C853">**Recording**</span> (green),
  <span style="color:#FFD600">**Online**</span> (yellow),
  <span style="color:#7A7A7A">**Offline**</span> (gray).
- **Timetable** (right) shows the next upcoming set per stage with `Stage | Time | Artist | Starts In`.
- Stage names appear in their real color (RED, BLUE, MAGENTA, UV, …).

## 🛠️ Tech Stack

- **Language:** Go (1.26+)
- **TUI:** [tview](https://github.com/rivo/tview) / [tcell](https://github.com/gdamore/tcell)
- **Recording:** [yt-dlp](https://github.com/yt-dlp/yt-dlp) + [FFmpeg](https://ffmpeg.org/)

## 🏗️ Architecture

The codebase follows a strict separation of concerns — the UI only renders
snapshots produced by the domain packages.

```
cmd/recorder/        Entrypoint: wiring, signal handling, graceful shutdown
internal/
  config/            Environment-driven configuration & channel list
  logging/           Minimal Logger interface (no silent failures)
  mixlr/             Mixlr JSON:API client (live flag + broadcast stream URL)
  timetable/         Timetable loader, current & upcoming set queries
  recorder/          yt-dlp process manager, stalled monitor, shutdown
  controller/        Channel-check & stalled-monitor scheduling loops
  status/            Per-channel stream state (online/offline) registry
  tools/             Bundled yt-dlp / ffmpeg discovery (bundled dir → PATH)
  tui/               tview dashboard (tables, logs, status) + channel logger
  util/              Shared helpers (formatting, sanitization, timezone)
```

Data flow: `Controller → Mixlr client → Recorder → yt-dlp`, with the `status`
registry feeding the TUI and the `timetable` enriching artist/ends-in columns.

## 🚀 Getting Started

### Runtime prerequisites

**Prebuilt releases bundle yt-dlp and FFmpeg** — extract and run, nothing else to install.

If you build from source (or want to use your own copies), the app looks for
`yt-dlp` and `ffmpeg` next to its own binary (or in `TOOLS_DIR`) first, then
falls back to your `PATH`. Install them only if you are not using a bundled release:

- [yt-dlp](https://github.com/yt-dlp/yt-dlp) — stream downloading
- [FFmpeg](https://ffmpeg.org/) — audio conversion (MP3)

### Option A — Prebuilt binary (no Go required)

Grab a release archive from `dist/` (produced by `make release`, see below) for
your platform, extract it and run:

```bash
# macOS / Linux
tar -xzf defqon-recorder-*-darwin-arm64.tar.gz
cd defqon-recorder-*-darwin-arm64
./defqon-recorder-darwin-arm64
```

```powershell
# Windows (PowerShell)
Expand-Archive defqon-recorder-*-windows-amd64.zip
cd defqon-recorder-*-windows-amd64
.\defqon-recorder-windows-amd64.exe
```

Each archive is fully self-contained: it bundles the binary, **yt-dlp, FFmpeg**
and `dq-timetable.json`, so it runs out of the box with zero external installs.

### Option B — Build from source

```bash
git clone https://github.com/revunix/DEFQON.1-Recorder.git
cd DEFQON.1-Recorder
make run        # builds and launches the recorder
```

Or with plain Go:

```bash
go build -o defqon-recorder ./cmd/recorder
./defqon-recorder
```

## 📦 Cross-platform releases

Build static binaries for all platforms and bundle distributable archives:

```bash
make release
```

This produces the following in `dist/` (CGO disabled → fully static). Archives
bundle yt-dlp + FFmpeg so they are self-contained:

| Platform            | Binary                              | Archive     |
|---------------------|-------------------------------------|-------------|
| macOS (Apple Silicon) | `defqon-recorder-darwin-arm64`    | `.tar.gz`   |
| macOS (Intel)         | `defqon-recorder-darwin-amd64`    | `.tar.gz`   |
| Linux (x86-64)        | `defqon-recorder-linux-amd64`     | `.tar.gz`   |
| Linux (arm64)         | `defqon-recorder-linux-arm64`     | `.tar.gz`   |
| Windows (x86-64)      | `defqon-recorder-windows-amd64.exe` | `.zip`    |
| Windows (arm64)       | `defqon-recorder-windows-arm64.exe` | `.zip`    |

The version in the archive name is derived from `git describe` — set a tag
(e.g. `git tag v1.0.0`) before releasing for clean versioned names.

## 🐳 Docker

```bash
make docker
docker run --rm -it -v "$PWD/recordings:/app/recordings" defqon-recorder
```

The image bundles yt-dlp and FFmpeg, so only Docker is required to run it.
Mount a volume to persist your recordings.

## 🎛️ Controls

| Key          | Action                                              |
|--------------|-----------------------------------------------------|
| `l`          | Listen to the selected live stream inside the TUI   |
| `s`          | Stop TUI audio playback                             |
| `q`          | Quit (graceful shutdown of all recordings)          |
| `Ctrl+C`     | Quit (graceful shutdown of all recordings)          |
| `SIGTERM`    | Graceful shutdown (e.g. `docker stop`)              |

## 📂 File Naming

Recordings are saved as MP3 in the configured directory:

```
[StageName]_[YYYY-MM-DDThh-mm-ss].mp3
```

Example: `BLUE_2026-06-26T18-00-00.000Z.mp3`

## 🛠️ Configuration

The application works with sensible defaults — no configuration required.

### Environment Variables

| Variable               | Description                                  | Default             |
|------------------------|----------------------------------------------|---------------------|
| `RECORDINGS_DIR`       | Directory to save recordings                 | `./recordings`      |
| `TIMETABLE_PATH`       | Path to the timetable JSON                   | `dq-timetable.json` |
| `TOOLS_DIR`            | Directory with bundled yt-dlp / ffmpeg       | exe directory       |
| `TEST_MIXLR_CHANNEL`   | Optional extra Mixlr channel slug for testing | unset               |
| `CHECK_INTERVAL_MS`    | Stream check interval (ms)                   | `60000`             |
| `TUI_UPDATE_INTERVAL_MS` | UI refresh rate (ms)                        | `2000`              |

## 🧪 Testing

```bash
make check       # fmt + vet + test
go test ./...    # tests only
```

## 🔧 Make Targets

```bash
make help
```

| Target    | Description                                          |
|-----------|------------------------------------------------------|
| `build`   | Compile the local binary                             |
| `run`     | Build and launch                                     |
| `release` | Cross-compile all platforms + bundle archives        |
| `check`   | Format, vet and test                                 |
| `test`    | Run unit tests                                       |
| `fmt`     | Format sources (`gofmt -s`)                          |
| `vet`     | Static analysis                                      |
| `tidy`    | Tidy module dependencies                             |
| `docker`  | Build the container image                            |
| `clean`   | Remove build artifacts                               |

## 📝 License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Made with care for the DEFQON.1 community
- Powered by [Mixlr](https://mixlr.com/)
- Built with [Go](https://go.dev/) and [tview](https://github.com/rivo/tview)
- Timetable data based on work by [codecat](https://github.com/codecat)

---

*This project is not affiliated with or endorsed by Q-dance or Mixlr. Use at your own risk and respect all copyright laws and terms of service.*

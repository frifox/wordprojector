# Word Projector

Word Projector is a cross-platform church projection program: Bible verses,
songs, picture slide shows, audio/video and announcements on one or more
projector screens, controlled from a familiar operator window.

It is a re-implementation of the abandoned [SoftProjector](http://softprojector.org)
(Qt/C++) with a **Go** backend, a **React** user interface and **SQLite**
storage. It keeps full compatibility with SoftProjector data:

* the database layout is the same (`spData.sqlite`, version 2), so an existing
  SoftProjector database can be used as is;
* songbooks (`.sps`), Bibles (`.spb`), themes (`.spt`) and schedules (`.spsc`)
  are imported and exported in the original formats;
* Bibles and songbooks are downloaded from the original softprojector.org
  module servers ("Download && Import" in Manage Database).

## Download

Standalone builds for macOS, Windows and Linux are published on the
[GitHub Releases](../../releases) page. Nothing needs to be installed:

| OS      | File                                   | Notes |
|---------|----------------------------------------|-------|
| macOS   | `WordProjector-<ver>-macos-arm64.zip` (Apple Silicon) / `...-macos-x64.zip` (Intel) | Unzip, move to Applications. The app is not notarized: right-click → Open the first time, or run `xattr -cr WordProjector.app`. |
| Windows | `WordProjector-<ver>-windows-x64.zip` | Unzip and run `WordProjector.exe`. Uses the WebView2 runtime that ships with Windows 10/11. |
| Linux   | `WordProjector-<ver>-linux-x64.tar.gz` | Untar and run `./WordProjector`. Needs WebKitGTK 4.1 (`libwebkit2gtk-4.1-0` on Debian/Ubuntu), present on most desktops. |

Data (database, downloaded modules, log) lives in the per-user application
data folder:

* macOS: `~/Library/Application Support/WordProjector`
* Windows: `%APPDATA%\WordProjector`
* Linux: `~/.local/share/WordProjector`

Put a `spData.sqlite` next to the executable for a portable installation, or
set `WORDPROJECTOR_DATA=<dir>` / `--data <dir>`.

## Features

* **Bible** tab: book/chapter/verse browsing with quick entry ("1King 3 13"),
  full text search (phrase, whole words, verse begins, any/all words),
  multi-verse display, up to three parallel translations per screen, history.
* **Songs** tab: songbooks, categories, filter by number/title, full text
  search with highlighting, automatic verse/refrain ordering from the
  SoftProjector song format, song editor with private per-song styling and
  background, song usage counter.
* **Pictures** tab: slide shows stored in the database, image resizing options.
* **Media** tab: audio/video library with preview and playback controls on
  the projector.
* **Announcements** tab: multi-slide announcements with timed auto advance.
* **Service schedule**: build, reorder, save and open `.spsc` schedules.
* **Themes**: per-screen fonts, colours, shadows, fading, alignment, captions,
  backgrounds, screen usage; themes are importable/exportable.
* **Displays**: single screen mode (display window appears when something is
  shown, with on-screen prev/next/exit controls) or up to four separate
  projector screens. Any browser on the network can also act as a display by
  opening `http://<computer>:<port>/projector/1` (turn on "Enable Public Access" in Settings > Remote, or run with `--listen 0.0.0.0:8777`).
* Printing of chapters, songs, announcements and schedules; light/dark UI;
  live preview of the projector output in the operator window.

## Building from source

Requirements: Go 1.26+, Node 22+. Linux additionally needs
`libgtk-3-dev libwebkit2gtk-4.1-dev`; macOS needs the Xcode command line tools.

```bash
make build        # builds the frontend and bin/wordprojector (desktop app)
make headless     # server-only binary (no native windows, no cgo)
make test
```

Development with hot reload:

```bash
make dev          # Go API on :8777 and Vite on :5173
```

The desktop app can also run as a plain web server: `wordprojector --headless --listen 127.0.0.1:8777`.

Releases are built by the GitHub Actions workflow in
`.github/workflows/release.yml` when a `v*` tag is pushed.

## Architecture

* `cmd/wordprojector` – entry point (desktop and headless variants).
* `internal/db`, `internal/qt` – SoftProjector compatible SQLite schema and
  Qt value encodings (QFont strings, QColor integers).
* `internal/songs`, `internal/bible`, `internal/announce`, `internal/slides`,
  `internal/media`, `internal/theme`, `internal/settings` – data model.
* `internal/modules` – `.sps` / `.spb` / `.spt` import & export and module
  downloads; `internal/schedule` – `.spsc` files.
* `internal/live` – projection state machine producing a resolved "slide"
  description per display.
* `internal/server` – HTTP + WebSocket API and the embedded UI.
* `internal/shell` – Wails v3 windows (operator window, projector windows,
  native dialogs).
* `frontend/` – React/TypeScript operator UI and projector renderer.

## License

GNU General Public License v3, like SoftProjector. Icons and data formats are
from SoftProjector, © Vladislav Kobzar and contributors.

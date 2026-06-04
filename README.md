<div align="center">
  <img src="build/appicon.png" width="120" alt="DMS icon"/>

  # DMS

  A desktop GUI for [**anacrolix/dms**](https://github.com/anacrolix/dms) — the UPnP/DLNA media server.
</div>

DMS wraps the [`anacrolix/dms`](https://github.com/anacrolix/dms) command-line tool in a simple
desktop app so you can start and stop a UPnP/DLNA media server from a window instead of the
terminal. Built with [Wails](https://wails.io) (Go + WebView) and a React + Mantine frontend.

The dms server is **embedded in-process as a Go library**, so DMS is a single self-contained
binary — no separate `dms` install required.

## Features

- Pick a media folder and serve it over UPnP/DLNA with one click.
- Configure HTTP port, friendly name, SSDP network interface, and allowed client IPs.
- Toggles: ignore hidden, ignore unreadable, no ffprobe, no transcode.
- **Additional options:** device icon (PNG shown in DLNA clients), persistent ffprobe cache
  file (`-fFprobeCachePath`), and forced transcode target (`-forceTranscodeTo`:
  chromecast / vp8 / web).
- Detects `ffmpeg` / `ffprobe` on `PATH` and disables transcoding/probing when they are missing.
- Collapsible live server log; window auto-fits its content.

## Requirements

- For full media probing and transcoding, install [`ffmpeg`](https://ffmpeg.org) (provides
  `ffmpeg` and `ffprobe`). DMS works without it for plain file serving.

## Development

```sh
wails3 dev
```

Runs a Vite dev server with hot reload. Right-click the window and choose *Inspect* to open
the webview devtools, where the bound Go methods are also reachable.

## Building

```sh
wails3 build          # binary into bin/
wails3 task package   # platform installer/bundle (AppImage, NSIS, .app)
```

Produces a redistributable binary in `bin/`. CI (`.github/workflows/build.yml`) builds a
Linux AppImage, a Windows `.exe`, and a macOS universal `.app`; pushing a `v*` tag publishes a
GitHub release with all three.

## License

[MIT](LICENSE) © zeeshan

The embedded media server is [`anacrolix/dms`](https://github.com/anacrolix/dms), licensed
separately by its authors.

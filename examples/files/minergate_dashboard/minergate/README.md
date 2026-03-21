# MinerGate Dashboard v1.0.3

A professional mining dashboard GUI for Antminer-compatible ASIC miners,
built with [Fyne](https://fyne.io) and
[go-echarts](https://github.com/go-echarts/go-echarts).

---

## Highlights

| Feature | Detail |
|---|---|
| **Real-time stats** | Hashrate, temperature, fan speed, power — polled every N seconds |
| **Interactive chart** | go-echarts line chart with zoom, per-chain series, served from an in-process HTTP server and opened in the system browser |
| **Inline sparkline** | Always-visible mini chart drawn with native Fyne canvas primitives |
| **Time-range filter** | 1 h / 6 h / 24 h / 7 d / All |
| **CSV history** | Hashrate automatically appended to `~/.config/minergate/data/total_hashrate.csv` |
| **Multi-miner** | Add any number of miner targets via Settings → Add Miner |
| **Pool & warnings** | Aggregated pool list and warning table from all miners |
| **Dark / light theme** | Configured in `config.json` |

---

## Requirements

* Go 1.21+
* C compiler + OpenGL (see [Fyne getting started](https://docs.fyne.io/started/))

### Linux

```bash
sudo apt-get install gcc libgl1-mesa-dev xorg-dev
```

### macOS

```bash
xcode-select --install
```

### Windows

Install [MinGW-w64](https://www.mingw-w64.org/) or Visual Studio Build Tools.

---

## Quick start

```bash
git clone https://github.com/Seif-Abaza/GUI_MinerGate.git
cd GUI_MinerGate

make deps
make build
./build/bin/minergate
```

Or simply:

```bash
make run
```

---

## Configuration

The default config is read from `~/.config/minergate/config.json`
(or `%APPDATA%\minergate\config.json` on Windows).
You can override it with `-config /path/to/config.json`.

```json
{
    "language": "en",
    "auto_refresh": true,
    "refresh_rate": 15,
    "theme": "dark",
    "miners": [
        {
            "name": "My S19 Pro",
            "host": "192.168.1.100",
            "port": 80,
            "api_port": 4028,
            "enabled": true
        }
    ]
}
```

---

## Project structure

```
minergate/
├── cmd/minergate/main.go           Entry point
├── internal/
│   ├── api/client.go               HTTP client for cgminer CGI endpoints
│   ├── charts/charts.go            CSV persistence + go-echarts HTML generation
│   │                               + ChartServer (in-process HTTP)
│   ├── config/config.go            Config load/save
│   ├── gui/dashboard.go            Fyne UI – Dashboard + ChartWidget + sparkline
│   └── models/models.go            Shared data models
├── config.json                     Example configuration
├── go.mod
└── Makefile
```

---

## How the chart works

1. Each poll cycle, the total hashrate (sum across all online miners) is
   appended to `<DataDir>/total_hashrate.csv`.
2. `charts.ReadAll()` reads the CSV and returns `[]models.HashRatePoint`.
3. `charts.GenerateHTML()` uses **go-echarts v2** to build a self-contained
   HTML page with a dark-themed, zoomable line chart.
4. `charts.ChartServer` serves this HTML from `http://localhost:<random-port>/chart`.
5. Clicking **📊 Open Interactive Chart** calls `xdg-open` / `open` / `start`
   to launch the URL in the system browser.
6. The sparkline inside the Fyne window is drawn independently with
   `canvas.Line` objects — no browser required.

---

## Adding go-echarts

```bash
go get github.com/go-echarts/go-echarts/v2@latest
```

---

## License

MIT © Seif Abaza

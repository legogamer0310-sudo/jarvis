# J.A.R.V.I.S. for Windows

A small Windows app for Pon's JARVIS assistant: a floating glowing bubble that stays on top of every window, and a JARVIS window showing the web JARVIS (signed in to Claude, no API key).

## Install

Download **[JARVIS.exe](dist/JARVIS.exe)** (click it, then the download button) and double-click it. It installs itself, adds Desktop and Start menu icons, and starts with Windows.

## Updates

JARVIS reads `version.txt` here a little after he starts and every hour. When it names a newer version, he downloads `dist/JARVIS.exe`, quietly, and switches to it the next time his window is closed.

## Building

```
go install github.com/tc-hib/go-winres@latest
go-winres make --arch amd64
GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui -s -w" -o JARVIS.exe .
```

To ship an update: raise `appVersion` in `updater.go`, build, copy the new `JARVIS.exe` into `dist/`, and put the same version in `version.txt`.

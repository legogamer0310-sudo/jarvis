# J.A.R.V.I.S. for Windows

A small Windows app for Pon's JARVIS assistant: a floating glowing bubble that stays on top of every window, and a JARVIS window showing the web JARVIS (signed in to Claude, no API key).

## Install

Download **JARVIS.exe** from the [latest release](../../releases/latest) and double-click it. It installs itself, adds Desktop and Start menu icons, and starts with Windows.

## Updates

JARVIS checks this page's latest release a little after it starts and every 6 hours. When there is a newer version, he downloads it quietly and switches to it the next time his window is closed.

## Building

```
go install github.com/tc-hib/go-winres@latest
go-winres make --arch amd64
GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui -s -w" -o JARVIS.exe .
```

To ship an update: raise `appVersion` in `updater.go`, build, and publish a GitHub release tagged `v<version>` with `JARVIS.exe` attached.

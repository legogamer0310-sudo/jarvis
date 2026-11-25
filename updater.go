package main

// Self-update: JARVIS checks his GitHub page for a newer version, downloads it
// quietly, and swaps it in the next time his window is closed.

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const appVersion = "4.2.0"

// updateRepo is the GitHub "owner/repo" that holds JARVIS releases.
const updateRepo = "legogamer0310-sudo/jarvis"

var (
	updMu    sync.Mutex
	updReady string // path of a downloaded newer JARVIS.exe
)

func verParts(v string) [3]int {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	for i, p := range strings.SplitN(v, ".", 3) {
		n, _ := strconv.Atoi(strings.TrimFunc(p, func(r rune) bool { return r < '0' || r > '9' }))
		out[i] = n
	}
	return out
}

func newer(a, b string) bool { // is a newer than b?
	x, y := verParts(a), verParts(b)
	for i := 0; i < 3; i++ {
		if x[i] != y[i] {
			return x[i] > y[i]
		}
	}
	return false
}

// checkUpdate reads version.txt from the GitHub page; if it names a newer
// version, it downloads dist/JARVIS.exe next to the installed copy.
func checkUpdate() {
	if strings.HasPrefix(updateRepo, "OWNER/") {
		return
	}
	base := "https://raw.githubusercontent.com/" + updateRepo + "/main/"
	get := func(url string, timeout time.Duration) (*http.Response, error) {
		cl := http.Client{Timeout: timeout}
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("User-Agent", "JARVIS-app")
		req.Header.Set("Cache-Control", "no-cache")
		return cl.Do(req)
	}
	r, err := get(base+"version.txt?t="+strconv.FormatInt(time.Now().Unix(), 10), 30*time.Second)
	if err != nil {
		return
	}
	b, _ := io.ReadAll(io.LimitReader(r.Body, 64))
	r.Body.Close()
	if r.StatusCode != 200 || !newer(string(b), appVersion) {
		return
	}
	resp, err := get(base+"dist/JARVIS.exe", 10*time.Minute)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return
	}
	dst := filepath.Join(installDir(), "JARVIS.new.exe")
	f, err := os.Create(dst)
	if err != nil {
		return
	}
	n, err := io.Copy(f, resp.Body)
	f.Close()
	head := make([]byte, 2)
	if g, e := os.Open(dst); e == nil {
		g.Read(head)
		g.Close()
	}
	if err != nil || n < 500_000 || (resp.ContentLength > 0 && n != resp.ContentLength) || string(head) != "MZ" {
		os.Remove(dst)
		return
	}
	updMu.Lock()
	updReady = dst
	updMu.Unlock()
}

// maybeApplyUpdate swaps in a downloaded update when JARVIS's window is closed.
func maybeApplyUpdate() {
	updMu.Lock()
	p := updReady
	updMu.Unlock()
	if p == "" || panelVisible() {
		return
	}
	exe := exePath()
	old := filepath.Join(installDir(), "JARVIS.old.exe")
	os.Remove(old)
	if os.Rename(exe, old) != nil {
		return
	}
	if os.Rename(p, exe) != nil {
		os.Rename(old, exe)
		return
	}
	updMu.Lock()
	updReady = ""
	updMu.Unlock()
	launch(exe, "--background --updated")
	pDestroyWindow.Call(hwnd)
}

func startUpdater() {
	os.Remove(filepath.Join(installDir(), "JARVIS.old.exe"))
	os.Remove(filepath.Join(installDir(), "JARVIS.new.exe"))
	if !strings.EqualFold(filepath.Clean(exePath()), filepath.Clean(filepath.Join(installDir(), "JARVIS.exe"))) {
		return
	}
	go func() {
		time.Sleep(20 * time.Second)
		for {
			checkUpdate()
			time.Sleep(6 * time.Hour)
		}
	}()
}

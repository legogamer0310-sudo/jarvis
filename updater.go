package main

// Self-update: JARVIS checks his GitHub page for a newer version, downloads it
// quietly, and swaps it in the next time his window is closed.

import (
	"encoding/json"
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

func checkUpdate() {
	if strings.HasPrefix(updateRepo, "OWNER/") {
		return
	}
	cl := http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequest("GET", "https://api.github.com/repos/"+updateRepo+"/releases/latest", nil)
	req.Header.Set("User-Agent", "JARVIS-app")
	req.Header.Set("Accept", "application/vnd.github+json")
	r, err := cl.Do(req)
	if err != nil {
		return
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return
	}
	var rel struct {
		Tag    string `json:"tag_name"`
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
			Size int64  `json:"size"`
		} `json:"assets"`
	}
	if json.NewDecoder(r.Body).Decode(&rel) != nil || !newer(rel.Tag, appVersion) {
		return
	}
	for _, a := range rel.Assets {
		if !strings.EqualFold(a.Name, "JARVIS.exe") {
			continue
		}
		dl := http.Client{Timeout: 10 * time.Minute}
		req, _ := http.NewRequest("GET", a.URL, nil)
		req.Header.Set("User-Agent", "JARVIS-app")
		resp, err := dl.Do(req)
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
		if err != nil || n < 500_000 || (a.Size > 0 && n != a.Size) || string(head) != "MZ" {
			os.Remove(dst)
			return
		}
		updMu.Lock()
		updReady = dst
		updMu.Unlock()
		return
	}
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

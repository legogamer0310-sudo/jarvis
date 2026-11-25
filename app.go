package main

// Installing, updating and "only one JARVIS at a time".

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	wmApp      = 0x8000
	wmShowMain = wmApp + 5
)

var (
	background   bool
	pCloseHandle = kernel32.NewProc("CloseHandle")
	pFindWindow  = user32.NewProc("FindWindowW")
)

func installDir() string {
	d := os.Getenv("LOCALAPPDATA")
	if d == "" {
		d = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
	}
	return filepath.Join(d, "Programs", "JARVIS")
}

// closeClass asks every running window of this class (an older or other copy) to close.
func closeClass(class string) bool {
	found := false
	for tries := 0; tries < 40; tries++ {
		w, _, _ := pFindWindow.Call(uintptr(unsafe.Pointer(u16(class))), 0)
		if w == 0 {
			break
		}
		found = true
		if tries%10 == 0 {
			pPostMessage.Call(w, wmClose, 0, 0)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return found
}

func removeRunValue(name string) {
	k := openRunKey()
	if k == 0 {
		return
	}
	defer pRegCloseKey.Call(k)
	pRegDeleteValue.Call(k, uintptr(unsafe.Pointer(u16(name))))
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	var err error
	for i := 0; i < 15; i++ {
		err = func() error {
			in, err := os.Open(src)
			if err != nil {
				return err
			}
			defer in.Close()
			tmp := dst + ".new"
			out, err := os.Create(tmp)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, in); err != nil {
				out.Close()
				return err
			}
			out.Close()
			os.Remove(dst)
			return os.Rename(tmp, dst)
		}()
		if err == nil {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return err
}

const shortcutScript = `$t = $env:JARVIS_TARGET
$sh = New-Object -ComObject WScript.Shell
foreach ($dir in @([Environment]::GetFolderPath('Desktop'), [Environment]::GetFolderPath('Programs'))) {
  if ($dir) {
    $l = $sh.CreateShortcut((Join-Path $dir 'JARVIS.lnk'))
    $l.TargetPath = $t
    $l.WorkingDirectory = Split-Path $t
    $l.IconLocation = "$t,0"
    $l.Description = 'J.A.R.V.I.S.'
    $l.Save()
  }
}`

func makeShortcuts(target string) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", shortcutScript)
	cmd.Env = append(os.Environ(), "JARVIS_TARGET="+target)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	done := make(chan struct{})
	go func() { cmd.Run(); close(done) }()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
	}
}

func launch(path string, args string) {
	var a uintptr
	if args != "" {
		a = uintptr(unsafe.Pointer(u16(args)))
	}
	pShellExecute.Call(0, uintptr(unsafe.Pointer(u16("open"))), uintptr(unsafe.Pointer(u16(path))), a, 0, 1)
}

// startup returns true if this process should go on and show JARVIS.
func startup() bool {
	reset, waitOld := false, false
	for _, a := range os.Args[1:] {
		if a == "--background" {
			background = true
		}
		if a == "--reset" || a == "--updated" {
			reset = a == "--reset"
			waitOld = true
		}
	}
	if waitOld {
		closeClass("JarvisOrbWnd") // the copy that started us is closing
	}
	if reset {
		// "Sign in again": forget the saved sign-in and browser settings
		// so the Claude sign-in page comes back.
		dir := filepath.Join(dataDir(), "browser")
		for i := 0; i < 40; i++ {
			if os.RemoveAll(dir) == nil {
				if _, err := os.Stat(dir); err != nil {
					break
				}
			}
			time.Sleep(250 * time.Millisecond)
		}
	}

	// Retire the earlier "JARVIS Bubble" versions.
	closeClass("JarvisBubbleWnd")
	removeRunValue("JARVIS Bubble")

	exe := exePath()
	target := filepath.Join(installDir(), "JARVIS.exe")
	if !strings.EqualFold(filepath.Clean(exe), filepath.Clean(target)) {
		// Running from Downloads (or anywhere else): install into the user's Programs
		// folder, add Desktop and Start menu shortcuts, then start the installed copy.
		closeClass("JarvisOrbWnd")
		if err := copyFile(exe, target); err == nil {
			makeShortcuts(target)
			launch(target, "")
			return false
		}
		// Could not install: just run from here.
	}

	// Only one JARVIS at a time: if one is running, open its window instead.
	var e uintptr
	for i := 0; i < 40; i++ {
		h, _, _ := pCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(u16("Local\\JarvisAppSingleton"))))
		e, _, _ = pGetLastError.Call()
		if e != errAlreadyExists || !waitOld {
			break
		}
		pCloseHandle.Call(h)
		time.Sleep(250 * time.Millisecond)
	}
	if e == errAlreadyExists {
		if !background {
			if w, _, _ := pFindWindow.Call(uintptr(unsafe.Pointer(ourClass)), 0); w != 0 {
				pAllowSetForeground.Call(^uintptr(0))
				pPostMessage.Call(w, wmShowMain, 0, 0)
			}
		}
		return false
	}

	marker := filepath.Join(dataDir(), "app-installed.txt")
	if _, err := os.Stat(marker); err != nil {
		os.WriteFile(marker, []byte("yes"), 0o644)
		setAutostart(true)
	} else if autostartOn() {
		setAutostart(true) // keep the path current
	}
	return true
}

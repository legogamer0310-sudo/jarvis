package main

// The main JARVIS window. It shows the web JARVIS (claude.ai) using the
// Microsoft Edge WebView2 engine built into Windows, so no API key is needed:
// you sign in to Claude once and it remembers you.

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/jchv/go-webview2/pkg/edge"
)

const jarvisURL = "https://claude.ai/artifact/PCD7Np6RM7zuGNH29gaBUY"

var (
	pPostMessage      = user32.NewProc("PostMessageW")
	pIsWindowVis      = user32.NewProc("IsWindowVisible")
	pGetDpiForSystem  = user32.NewProc("GetDpiForSystem")
	pBringToTop       = user32.NewProc("BringWindowToTop")
	pGetForegroundWnd = user32.NewProc("GetForegroundWindow")
	pLoadImage        = user32.NewProc("LoadImageW")
	pCreateBrush      = gdi32.NewProc("CreateSolidBrush")
)

const (
	wsOverlappedWindow = 0x00CF0000
	wmClose            = 0x0010
	wmSize             = 0x0005
	wmMove             = 0x0003
	wmActivate         = 0x0006
	swHide             = 0
	hwndNoTopmost      = ^uintptr(1) // -2
)

var (
	panel      uintptr
	browser    *edge.Chromium
	embedded   bool
	thinking   bool // the page does the thinking; the orb just idles
	onTop      bool
	scale      = 1.0
	panelClass = u16("JarvisMainWnd")
)

func px(v float64) int32 { return int32(v*scale + 0.5) }

func createPanel(inst uintptr) {
	if pGetDpiForSystem.Find() == nil {
		if d, _, _ := pGetDpiForSystem.Call(); d >= 96 {
			scale = float64(d) / 96
		}
	}
	bg, _, _ := pCreateBrush.Call(0x0C0703)                                                        // near black
	icon, _, _ := pLoadImage.Call(inst, uintptr(unsafe.Pointer(u16("APP"))), 1, 0, 0, 0x40|0x8000) // IMAGE_ICON, LR_DEFAULTSIZE|LR_SHARED
	small, _, _ := pLoadImage.Call(inst, uintptr(unsafe.Pointer(u16("APP"))), 1, 16, 16, 0x8000)
	wc := wndClassEx{WndProc: syscall.NewCallback(panelProc), Instance: inst, ClassName: panelClass, Background: bg, Icon: icon, IconSm: small}
	wc.Size = uint32(unsafe.Sizeof(wc))
	c, _, _ := pLoadCursor.Call(0, 32512)
	wc.Cursor = c
	pRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))

	var work rect
	pSystemParametersInfo.Call(0x0030, 0, uintptr(unsafe.Pointer(&work)), 0)
	w, h := px(1180), px(760)
	if w > work.R-work.L-px(40) {
		w = work.R - work.L - px(40)
	}
	if h > work.B-work.T-px(40) {
		h = work.B - work.T - px(40)
	}
	x := work.L + (work.R-work.L-w)/2
	y := work.T + (work.B-work.T-h)/2
	panel, _, _ = pCreateWindowEx.Call(0,
		uintptr(unsafe.Pointer(panelClass)), uintptr(unsafe.Pointer(u16("J.A.R.V.I.S."))), wsOverlappedWindow,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h), 0, 0, inst, 0)
}

// embed starts the Edge engine inside the window the first time it opens.
func embed() bool {
	if embedded {
		return true
	}
	browser = edge.NewChromium()
	browser.DataPath = filepath.Join(dataDir(), "browser")
	browser.SetPermission(edge.CoreWebView2PermissionKindMicrophone, edge.CoreWebView2PermissionStateAllow)
	browser.SetPermission(edge.CoreWebView2PermissionKindClipboardRead, edge.CoreWebView2PermissionStateAllow)
	if !browser.Embed(panel) {
		msgBox("JARVIS needs the Microsoft Edge WebView2 engine, which is missing on this computer.\n\n" +
			"Click OK to open the download page. Install the \"Evergreen Bootstrapper\", then open JARVIS again.")
		launch("https://developer.microsoft.com/microsoft-edge/webview2/", "")
		browser = nil
		return false
	}
	embedded = true
	// After signing in, claude.ai lands on its chat page. Send JARVIS's window
	// straight back to the JARVIS interface instead (sign-in pages are left alone).
	browser.Init(`(function(){try{
  if (window.top !== window || location.hostname !== 'claude.ai') return;
  var p = location.pathname;
  if (p === '/' || p.indexOf('/new') === 0 || p.indexOf('/chat') === 0 || p.indexOf('/recents') === 0 ||
      p.indexOf('/project') === 0 || p.indexOf('/onboarding') === 0 || p.indexOf('/settings') === 0) {
    location.replace('` + jarvisURL + `');
  }
}catch(e){}})();`)
	browser.Resize()
	browser.Navigate(jarvisURL)
	return true
}

func panelVisible() bool {
	v, _, _ := pIsWindowVis.Call(panel)
	return v != 0
}

func showPanel() {
	if ic, _, _ := pIsIconic.Call(panel); ic != 0 {
		pShowWindow.Call(panel, swRestore)
	} else {
		pShowWindow.Call(panel, swShow)
	}
	pBringToTop.Call(panel)
	pSetForegroundWindow.Call(panel)
	if embed() {
		browser.Resize()
		browser.Focus()
	}
}

func hidePanel() { pShowWindow.Call(panel, swHide) }

// togglePanel: the bubble opens JARVIS, or tucks him away if he's already in front.
func togglePanel() {
	fg, _, _ := pGetForegroundWnd.Call()
	ic, _, _ := pIsIconic.Call(panel)
	if panelVisible() && ic == 0 && fg == panel {
		hidePanel()
	} else {
		showPanel()
	}
}

func reloadPanel() {
	showPanel()
	if browser != nil {
		browser.Navigate(jarvisURL)
	}
}

func signInPage() {
	showPanel()
	if browser != nil {
		browser.Navigate("https://claude.ai/login")
	}
}

func setOnTop(v bool) {
	onTop = v
	z := hwndNoTopmost
	if v {
		z = hwndTopmost
	}
	pSetWindowPos.Call(panel, z, 0, 0, 0, 0, swpNoMove|swpNoSize)
}

func panelProc(h uintptr, msg uint32, wp, lp uintptr) uintptr {
	switch msg {
	case wmSize:
		if browser != nil {
			browser.Resize()
		}
	case wmMove:
		if browser != nil {
			browser.NotifyParentWindowPositionChanged()
		}
	case wmActivate:
		if wp&0xFFFF != 0 && browser != nil {
			browser.Focus()
		}
	case wmClose: // X tucks JARVIS into the bubble; Quit is in the bubble's right-click menu
		hidePanel()
		tip := filepath.Join(dataDir(), "close-tip.txt")
		if _, err := os.Stat(tip); err != nil {
			os.WriteFile(tip, []byte("yes"), 0o644)
			go msgBox("JARVIS is still here, in the glowing bubble.\n\n" +
				"• Click the bubble to bring him back.\n" +
				"• To quit completely, right-click the bubble and choose Quit JARVIS.")
		}
		return 0
	}
	r, _, _ := pDefWindowProc.Call(h, uintptr(msg), wp, lp)
	return r
}

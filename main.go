// JARVIS Bubble: a small glowing orb that floats above every window.
// Click it to ask JARVIS a question, drag it anywhere, right-click for options.
// Pure Win32, no installs. Starts with Windows automatically.
package main

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const size = 76 // window is size x size pixels

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	pRegisterClassEx      = user32.NewProc("RegisterClassExW")
	pCreateWindowEx       = user32.NewProc("CreateWindowExW")
	pDefWindowProc        = user32.NewProc("DefWindowProcW")
	pGetMessage           = user32.NewProc("GetMessageW")
	pTranslateMessage     = user32.NewProc("TranslateMessage")
	pDispatchMessage      = user32.NewProc("DispatchMessageW")
	pPostQuitMessage      = user32.NewProc("PostQuitMessage")
	pSetTimer             = user32.NewProc("SetTimer")
	pGetDC                = user32.NewProc("GetDC")
	pReleaseDC            = user32.NewProc("ReleaseDC")
	pUpdateLayeredWindow  = user32.NewProc("UpdateLayeredWindow")
	pSetWindowPos         = user32.NewProc("SetWindowPos")
	pGetCursorPos         = user32.NewProc("GetCursorPos")
	pSetCapture           = user32.NewProc("SetCapture")
	pReleaseCapture       = user32.NewProc("ReleaseCapture")
	pCreatePopupMenu      = user32.NewProc("CreatePopupMenu")
	pAppendMenu           = user32.NewProc("AppendMenuW")
	pTrackPopupMenu       = user32.NewProc("TrackPopupMenu")
	pDestroyMenu          = user32.NewProc("DestroyMenu")
	pSetForegroundWindow  = user32.NewProc("SetForegroundWindow")
	pShowWindow           = user32.NewProc("ShowWindow")
	pIsIconic             = user32.NewProc("IsIconic")
	pEnumWindows          = user32.NewProc("EnumWindows")
	pGetWindowText        = user32.NewProc("GetWindowTextW")
	pGetClassName         = user32.NewProc("GetClassNameW")
	pIsWindowVisible      = user32.NewProc("IsWindowVisible")
	pMessageBox           = user32.NewProc("MessageBoxW")
	pSystemParametersInfo = user32.NewProc("SystemParametersInfoW")
	pLoadCursor           = user32.NewProc("LoadCursorW")
	pSetCursor            = user32.NewProc("SetCursor")
	pDestroyWindow        = user32.NewProc("DestroyWindow")
	pSetProcessDPIAware   = user32.NewProc("SetProcessDPIAware")
	pGetSystemMetrics     = user32.NewProc("GetSystemMetrics")
	pAllowSetForeground   = user32.NewProc("AllowSetForegroundWindow")

	pCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	pCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	pSelectObject       = gdi32.NewProc("SelectObject")

	pGetModuleHandle = kernel32.NewProc("GetModuleHandleW")
	pCreateMutex     = kernel32.NewProc("CreateMutexW")
	pGetLastError    = kernel32.NewProc("GetLastError")

	pShellExecute = shell32.NewProc("ShellExecuteW")

	pRegCreateKeyEx  = advapi32.NewProc("RegCreateKeyExW")
	pRegSetValueEx   = advapi32.NewProc("RegSetValueExW")
	pRegQueryValueEx = advapi32.NewProc("RegQueryValueExW")
	pRegDeleteValue  = advapi32.NewProc("RegDeleteValueW")
	pRegCloseKey     = advapi32.NewProc("RegCloseKey")
)

const (
	wsPopup          = 0x80000000
	wsExTopmost      = 0x00000008
	wsExToolWindow   = 0x00000080
	wsExLayered      = 0x00080000
	wsExNoActivate   = 0x08000000
	wmDestroy        = 0x0002
	wmTimer          = 0x0113
	wmMouseMove      = 0x0200
	wmLButtonDown    = 0x0201
	wmLButtonUp      = 0x0202
	wmRButtonUp      = 0x0205
	wmSetCursor      = 0x0020
	wmMouseActivate  = 0x0021
	maNoActivate     = 3
	hwndTopmost      = ^uintptr(0) // -1
	swpNoSize        = 0x0001
	swpNoMove        = 0x0002
	swpNoActivate    = 0x0010
	ulwAlpha         = 2
	tpmReturnCmd     = 0x0100
	tpmRightButton   = 0x0002
	mfString         = 0x0000
	mfChecked        = 0x0008
	mfSeparator      = 0x0800
	swRestore        = 9
	swShow           = 5
	hkeyCurrentUser  = 0x80000001
	keyAllAccess     = 0xF003F
	regSz            = 1
	errAlreadyExists = 183
	idcHand          = 32649
)

type point struct{ X, Y int32 }
type sizeT struct{ CX, CY int32 }
type rect struct{ L, T, R, B int32 }
type msgT struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
	_       uint32
}
type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}
type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}
type blendFunction struct {
	BlendOp, BlendFlags, SourceConstantAlpha, AlphaFormat byte
}

var (
	hwnd     uintptr
	memDC    uintptr
	pixels   []byte
	pos      point
	hovering float64 // 0..1 eased
	pressing bool
	dragging bool
	downAt   point
	downPos  point
	start    = time.Now()
	ourClass = u16("JarvisOrbWnd")
)

func u16(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }

// ------------------------------------------------------------ settings

func dataDir() string {
	d := os.Getenv("APPDATA")
	if d == "" {
		d = "."
	}
	d = filepath.Join(d, "JARVIS-Bubble")
	os.MkdirAll(d, 0o755)
	return d
}

func loadPos() (point, bool) {
	b, err := os.ReadFile(filepath.Join(dataDir(), "position.txt"))
	if err != nil {
		return point{}, false
	}
	f := strings.Fields(string(b))
	if len(f) != 2 {
		return point{}, false
	}
	x, e1 := strconv.Atoi(f[0])
	y, e2 := strconv.Atoi(f[1])
	if e1 != nil || e2 != nil {
		return point{}, false
	}
	return point{int32(x), int32(y)}, true
}

func savePos() {
	os.WriteFile(filepath.Join(dataDir(), "position.txt"),
		[]byte(strconv.Itoa(int(pos.X))+" "+strconv.Itoa(int(pos.Y))), 0o644)
}

// ------------------------------------------------------------ start with Windows

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const runName = "JARVIS"

func openRunKey() uintptr {
	var k uintptr
	r, _, _ := pRegCreateKeyEx.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(u16(runKey))), 0, 0, 0, keyAllAccess, 0, uintptr(unsafe.Pointer(&k)), 0)
	if r != 0 {
		return 0
	}
	return k
}

func exePath() string {
	p, _ := os.Executable()
	return p
}

func autostartOn() bool {
	k := openRunKey()
	if k == 0 {
		return false
	}
	defer pRegCloseKey.Call(k)
	var n uint32
	r, _, _ := pRegQueryValueEx.Call(k, uintptr(unsafe.Pointer(u16(runName))), 0, 0, 0, uintptr(unsafe.Pointer(&n)))
	return r == 0
}

func setAutostart(on bool) {
	k := openRunKey()
	if k == 0 {
		return
	}
	defer pRegCloseKey.Call(k)
	if on {
		v, _ := syscall.UTF16FromString(`"` + exePath() + `" --background`)
		pRegSetValueEx.Call(k, uintptr(unsafe.Pointer(u16(runName))), 0, regSz, uintptr(unsafe.Pointer(&v[0])), uintptr(len(v)*2))
	} else {
		pRegDeleteValue.Call(k, uintptr(unsafe.Pointer(u16(runName))))
	}
}

// ------------------------------------------------------------ drawing

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

// ring returns coverage (0..1) of a ring of radius r and half-thickness w at distance d.
func ring(d, r, w float64) float64 { return clamp01(w + 0.5 - math.Abs(d-r)) }

func render() {
	t := time.Since(start).Seconds()
	c := float64(size) / 2
	pulse := 0.5 + 0.5*math.Sin(t*2.2)
	hv := hovering
	spin := t * 1.4
	if thinking {
		spin = t * 5
		pulse = 0.5 + 0.5*math.Sin(t*8)
	}
	coreR := 15.0 + 1.5*pulse + 2*hv
	if pressing {
		coreR -= 2
	}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)+0.5-c, float64(y)+0.5-c
			d := math.Hypot(dx, dy)
			ang := math.Atan2(dy, dx)

			var r, g, b, a float64 // straight colour + alpha, composited "over" in order
			over := func(cr, cg, cb, ca float64) {
				if ca <= 0 {
					return
				}
				na := ca + a*(1-ca)
				if na <= 0 {
					return
				}
				r = (cr*ca + r*a*(1-ca)) / na
				g = (cg*ca + g*a*(1-ca)) / na
				b = (cb*ca + b*a*(1-ca)) / na
				a = na
			}

			// soft outer glow
			if d < c {
				glow := math.Exp(-math.Pow((d-27)/7, 2)) * (0.35 + 0.25*pulse + 0.3*hv)
				over(0.0, 0.75, 1.0, glow)
			}
			// dark body disc
			body := clamp01(29.5 - d)
			over(0.02, 0.06, 0.12, body*0.92)
			// outer ring
			over(0.25, 0.85, 1.0, ring(d, 28, 1.2)*(0.85+0.15*hv))
			// spinning arc segments
			seg := math.Mod(ang+spin+4*math.Pi, math.Pi/3)
			if seg < math.Pi/3*0.62 {
				over(0.35, 0.9, 1.0, ring(d, 23, 1.4)*0.9)
			}
			seg2 := math.Mod(ang-spin*1.7+4*math.Pi, math.Pi/2)
			if seg2 < math.Pi/2*0.35 {
				over(0.5, 0.95, 1.0, ring(d, 19.5, 0.8)*0.7)
			}
			// glowing core
			if d < coreR+1 {
				k := clamp01(coreR + 0.5 - d)
				f := d / coreR
				over(0.55+0.45*(1-f), 0.92+0.08*(1-f), 1.0, k*(0.95-0.35*f))
			}
			// "J" glyph in the core
			jx, jy := dx, dy
			stem := math.Abs(jx-3) < 1.6 && jy > -7.5 && jy < 3
			top := math.Abs(jy+7.5) < 1.4 && jx > -3 && jx < 7
			hook := ring(math.Hypot(jx+0.5, jy-3), 3.5, 0.9) > 0 && jy > 3
			if stem || top || hook {
				over(0.02, 0.18, 0.32, 0.9)
			}

			i := (y*size + x) * 4
			pixels[i+0] = byte(b * a * 255) // premultiplied BGRA
			pixels[i+1] = byte(g * a * 255)
			pixels[i+2] = byte(r * a * 255)
			pixels[i+3] = byte(a * 255)
		}
	}
	present()
}

func present() {
	screen, _, _ := pGetDC.Call(0)
	sz := sizeT{size, size}
	src := point{0, 0}
	bf := blendFunction{0, 0, 255, 1}
	pUpdateLayeredWindow.Call(hwnd, screen, uintptr(unsafe.Pointer(&pos)), uintptr(unsafe.Pointer(&sz)),
		memDC, uintptr(unsafe.Pointer(&src)), 0, uintptr(unsafe.Pointer(&bf)), ulwAlpha)
	pReleaseDC.Call(0, screen)
}

// ------------------------------------------------------------ window

func cursor() point {
	var p point
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return p
}

func overOrb(p point) bool {
	dx := float64(p.X-pos.X) - size/2
	dy := float64(p.Y-pos.Y) - size/2
	return math.Hypot(dx, dy) < 30
}

func showMenu() {
	m, _, _ := pCreatePopupMenu.Call()
	defer pDestroyMenu.Call(m)
	pAppendMenu.Call(m, mfString, 1, uintptr(unsafe.Pointer(u16("Open JARVIS"))))
	pAppendMenu.Call(m, mfString, 4, uintptr(unsafe.Pointer(u16("Show the JARVIS interface"))))
	top := uintptr(mfString)
	if onTop {
		top |= mfChecked
	}
	pAppendMenu.Call(m, top, 6, uintptr(unsafe.Pointer(u16("Keep JARVIS on top of other windows"))))
	pAppendMenu.Call(m, mfString, 8, uintptr(unsafe.Pointer(u16("Go to the Claude sign-in page"))))
	pAppendMenu.Call(m, mfString, 7, uintptr(unsafe.Pointer(u16("Sign in again (reset JARVIS)"))))
	flags := uintptr(mfString)
	if autostartOn() {
		flags |= mfChecked
	}
	pAppendMenu.Call(m, flags, 2, uintptr(unsafe.Pointer(u16("Start when my computer starts"))))
	pAppendMenu.Call(m, mfSeparator, 0, 0)
	pAppendMenu.Call(m, mfString, 3, uintptr(unsafe.Pointer(u16("Quit JARVIS"))))
	p := cursor()
	pSetForegroundWindow.Call(hwnd)
	cmd, _, _ := pTrackPopupMenu.Call(m, tpmReturnCmd|tpmRightButton, uintptr(p.X), uintptr(p.Y), 0, hwnd, 0)
	switch cmd {
	case 1:
		showPanel()
	case 6:
		setOnTop(!onTop)
	case 4:
		reloadPanel()
	case 8:
		signInPage()
	case 7:
		launch(exePath(), "--reset")
		pDestroyWindow.Call(hwnd)
	case 2:
		setAutostart(!autostartOn())
	case 3:
		pDestroyWindow.Call(hwnd)
	}
}

func wndProc(h uintptr, msg uint32, wp, lp uintptr) uintptr {
	switch msg {
	case wmMouseActivate:
		return maNoActivate
	case wmSetCursor:
		c, _, _ := pLoadCursor.Call(0, idcHand)
		pSetCursor.Call(c)
		return 1
	case wmLButtonDown:
		pressing, dragging = true, false
		downAt, downPos = cursor(), pos
		pSetCapture.Call(h)
		return 0
	case wmMouseMove:
		if pressing {
			p := cursor()
			dx, dy := p.X-downAt.X, p.Y-downAt.Y
			if dragging || dx*dx+dy*dy > 25 {
				dragging = true
				pos = point{downPos.X + dx, downPos.Y + dy}
				present()
			}
		}
		return 0
	case wmLButtonUp:
		if pressing {
			pressing = false
			pReleaseCapture.Call()
			if dragging {
				savePos()
			} else {
				togglePanel()
			}
			dragging = false
		}
		return 0
	case wmRButtonUp:
		showMenu()
		return 0
	case wmTimer:
		target := 0.0
		if overOrb(cursor()) {
			target = 1
		}
		hovering += (target - hovering) * 0.25
		render()
		if wp == 2 { // keep it above other windows
			maybeApplyUpdate()
			pSetWindowPos.Call(h, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
		}
		return 0
	case wmShowMain:
		showPanel()
		return 0
	case wmDestroy:
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProc.Call(h, uintptr(msg), wp, lp)
	return r
}

func msgBox(text string) {
	pMessageBox.Call(0, uintptr(unsafe.Pointer(u16(text))), uintptr(unsafe.Pointer(u16("JARVIS"))), 0x40|0x40000)
}

func main() {
	runtime.LockOSThread()
	pSetProcessDPIAware.Call()

	if !startup() {
		return
	}

	// Start position: last saved, else bottom-right above the taskbar.
	var work rect
	pSystemParametersInfo.Call(0x0030, 0, uintptr(unsafe.Pointer(&work)), 0) // SPI_GETWORKAREA
	if p, ok := loadPos(); ok {
		vw, _, _ := pGetSystemMetrics.Call(78) // virtual screen width
		vh, _, _ := pGetSystemMetrics.Call(79)
		vx, _, _ := pGetSystemMetrics.Call(76)
		vy, _, _ := pGetSystemMetrics.Call(77)
		if int32(p.X) >= int32(vx)-size/2 && int32(p.Y) >= int32(vy)-size/2 &&
			int32(p.X) < int32(vx)+int32(vw)-size/2 && int32(p.Y) < int32(vy)+int32(vh)-size/2 {
			pos = p
		} else {
			pos = point{work.R - size - 24, work.B - size - 24}
		}
	} else {
		pos = point{work.R - size - 24, work.B - size - 24}
	}

	inst, _, _ := pGetModuleHandle.Call(0)
	wc := wndClassEx{WndProc: syscall.NewCallback(wndProc), Instance: inst, ClassName: ourClass}
	wc.Size = uint32(unsafe.Sizeof(wc))
	pRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
	hwnd, _, _ = pCreateWindowEx.Call(wsExTopmost|wsExToolWindow|wsExLayered|wsExNoActivate,
		uintptr(unsafe.Pointer(ourClass)), uintptr(unsafe.Pointer(u16("JARVIS Orb"))), wsPopup,
		uintptr(pos.X), uintptr(pos.Y), size, size, 0, 0, inst, 0)

	// 32-bit top-down pixel buffer
	screen, _, _ := pGetDC.Call(0)
	memDC, _, _ = pCreateCompatibleDC.Call(screen)
	pReleaseDC.Call(0, screen)
	bi := bitmapInfoHeader{Width: size, Height: -size, Planes: 1, BitCount: 32}
	bi.Size = uint32(unsafe.Sizeof(bi))
	var bits uintptr
	bmp, _, _ := pCreateDIBSection.Call(memDC, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	pSelectObject.Call(memDC, bmp)
	pixels = unsafe.Slice((*byte)(unsafe.Pointer(bits)), size*size*4)

	createPanel(inst)
	render()
	pShowWindow.Call(hwnd, 4) // SW_SHOWNOACTIVATE
	pSetTimer.Call(hwnd, 1, 33, 0)
	pSetTimer.Call(hwnd, 2, 2000, 0)
	startUpdater()
	if !background {
		showPanel()
	}

	var m msgT
	for {
		r, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}

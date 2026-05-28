package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	analog "github.com/anacrolix/log"

	dmslib "github.com/anacrolix/dms/dlna/dms"
	"github.com/anacrolix/dms/rrcache"
	rt "github.com/wailsapp/wails/v2/pkg/runtime"
)

const dmsVersion = "anacrolix/dms v1.7.2 (embedded)"

// Config mirrors the dms command-line flags that the GUI exposes.
type Config struct {
	Path             string `json:"path"`
	HTTPPort         string `json:"httpPort"`
	FriendlyName     string `json:"friendlyName"`
	Ifname           string `json:"ifname"`
	AllowedIps       string `json:"allowedIps"`
	DeviceIcon       string `json:"deviceIcon"`
	FFprobeCachePath string `json:"ffprobeCachePath"`
	ForceTranscodeTo string `json:"forceTranscodeTo"`
	IgnoreHidden     bool   `json:"ignoreHidden"`
	IgnoreUnreadable bool   `json:"ignoreUnreadable"`
	NoProbe          bool   `json:"noProbe"`
	NoTranscode      bool   `json:"noTranscode"`
}

// Tools reports which external media tools are available on PATH. dms needs
// ffprobe for media probing and ffmpeg for transcoding.
type Tools struct {
	Ffmpeg  bool `json:"ffmpeg"`
	Ffprobe bool `json:"ffprobe"`
}

// App struct holds the embedded dms server and Wails context.
type App struct {
	ctx       context.Context
	mu        sync.Mutex
	srv       *dmslib.Server
	running   bool
	logw      *logEmitter
	cache     *fileCache
	cachePath string
}

// NewApp creates a new App application struct
func NewApp() *App {
	a := &App{}
	a.logw = &logEmitter{app: a}
	return a
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// shutdown stops the dms server if the window is closed while running.
func (a *App) shutdown(ctx context.Context) {
	_ = a.StopServer()
}

// ServerInfo reports the embedded dms version (no external binary needed).
func (a *App) ServerInfo() string {
	return dmsVersion
}

// GetDefaults returns sensible starting values for the form.
func (a *App) GetDefaults() Config {
	home, _ := os.UserHomeDir()
	return Config{
		Path:             home,
		HTTPPort:         ":1338",
		FFprobeCachePath: filepath.Join(home, ".dms-ffprobe-cache"),
	}
}

// ListInterfaces returns the names of usable network interfaces for SSDP.
func (a *App) ListInterfaces() []string {
	out := []string{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, i := range ifaces {
		if i.Flags&net.FlagLoopback != 0 || i.Flags&net.FlagUp == 0 {
			continue
		}
		out = append(out, i.Name)
	}
	sort.Strings(out)
	return out
}

// SelectDirectory opens a native folder picker and returns the chosen path.
func (a *App) SelectDirectory() (string, error) {
	return rt.OpenDirectoryDialog(a.ctx, rt.OpenDialogOptions{
		Title: "Select media folder to share",
	})
}

// CheckTools reports whether ffmpeg/ffprobe are on PATH so the UI can disable
// transcoding/probing options when the tools are missing.
func (a *App) CheckTools() Tools {
	t := Tools{}
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		t.Ffmpeg = true
	}
	if _, err := exec.LookPath("ffprobe"); err == nil {
		t.Ffprobe = true
	}
	return t
}

// SelectIconFile opens a native picker limited to PNG images for the device icon.
func (a *App) SelectIconFile() (string, error) {
	return rt.OpenFileDialog(a.ctx, rt.OpenDialogOptions{
		Title:   "Select device icon (PNG)",
		Filters: []rt.FileFilter{{DisplayName: "PNG images (*.png)", Pattern: "*.png"}},
	})
}

// IsRunning reports whether the embedded server is currently active.
func (a *App) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running
}

// StartServer builds and starts an in-process dms server from cfg. The server
// runs in its own goroutine; lifecycle changes emit "dms:status" and all dms
// log output is forwarded to the frontend via "dms:log".
func (a *App) StartServer(cfg Config) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return fmt.Errorf("server already running")
	}
	if cfg.Path == "" {
		return fmt.Errorf("no media folder selected")
	}

	port := cfg.HTTPPort
	if port == "" {
		port = ":1338"
	}

	base, err := net.Listen("tcp", port)
	if err != nil {
		return fmt.Errorf("cannot listen on %q: %w", port, err)
	}

	// dms only enforces AllowedIpNets on the SOAP control endpoint, leaving
	// media streaming and the device descriptions open to any IP. Filter at the
	// accept layer instead so the allow-list covers every endpoint, and log each
	// connection so it is visible in the server log.
	allowAll := strings.TrimSpace(cfg.AllowedIps) == ""
	ipNets := makeIpNets(cfg.AllowedIps)
	listener := &ipFilterListener{
		Listener: base,
		nets:     ipNets,
		allowAll: allowAll,
		logf:     func(format string, args ...interface{}) { fmt.Fprintf(a.logw, ">>> "+format+"\n", args...) },
	}
	if allowAll {
		fmt.Fprintf(a.logw, ">>> allowed clients: ALL (no restriction)\n")
	} else {
		fmt.Fprintf(a.logw, ">>> allowed clients: %s\n", strings.TrimSpace(cfg.AllowedIps))
	}

	logger := analog.NewLogger("dms").WithFilterLevel(analog.Debug)
	logger.SetHandlers(analog.StreamHandler{W: a.logw, Fmt: analog.LineFormatter})

	cache := newFileCache()
	if cfg.FFprobeCachePath != "" {
		if err := cache.load(cfg.FFprobeCachePath); err != nil && !os.IsNotExist(err) {
			rt.EventsEmit(a.ctx, "dms:log", fmt.Sprintf(">>> ffprobe cache not loaded: %v", err))
		}
	}

	srv := &dmslib.Server{
		Logger:           logger,
		Interfaces:       pickInterfaces(cfg.Ifname),
		HTTPConn:         listener,
		FriendlyName:     cfg.FriendlyName,
		RootObjectPath:   filepath.Clean(cfg.Path),
		FFProbeCache:     cache,
		NoTranscode:      cfg.NoTranscode,
		ForceTranscodeTo: cfg.ForceTranscodeTo,
		NoProbe:          cfg.NoProbe,
		IgnoreHidden:     cfg.IgnoreHidden,
		IgnoreUnreadable: cfg.IgnoreUnreadable,
		AllowedIpNets:    makeIpNets(cfg.AllowedIps),
		NotifyInterval:   30 * time.Second,
	}

	if cfg.DeviceIcon != "" {
		if icons, err := buildIcons(cfg.DeviceIcon); err != nil {
			rt.EventsEmit(a.ctx, "dms:log", fmt.Sprintf(">>> device icon ignored: %v", err))
		} else {
			srv.Icons = icons
		}
	}

	if err := srv.Init(); err != nil {
		_ = listener.Close()
		return fmt.Errorf("failed to init dms server: %w", err)
	}

	a.srv = srv
	a.cache = cache
	a.cachePath = cfg.FFprobeCachePath
	a.running = true

	go func() {
		runErr := srv.Run()

		a.mu.Lock()
		a.running = false
		a.srv = nil
		path := a.cachePath
		a.mu.Unlock()

		if path != "" {
			if err := cache.save(path); err != nil {
				rt.EventsEmit(a.ctx, "dms:log", fmt.Sprintf(">>> ffprobe cache not saved: %v", err))
			}
		}

		msg := "Server stopped"
		if runErr != nil {
			msg = fmt.Sprintf("Server stopped: %v", runErr)
		}
		a.emitStatus(false, msg)
	}()

	a.emitStatus(true, fmt.Sprintf("Serving %q on %s", cfg.Path, port))
	return nil
}

// StopServer closes the embedded server, which unblocks its Run goroutine.
func (a *App) StopServer() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running || a.srv == nil {
		return nil
	}
	return a.srv.Close()
}

func (a *App) emitStatus(running bool, message string) {
	if a.ctx == nil {
		return
	}
	rt.EventsEmit(a.ctx, "dms:status", map[string]interface{}{
		"running": running,
		"message": message,
	})
}

// logEmitter is an io.Writer that splits dms log output into lines and forwards
// each completed line to the frontend as a "dms:log" event.
type logEmitter struct {
	app *App
	mu  sync.Mutex
	buf []byte
}

func (w *logEmitter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := string(bytes.TrimRight(w.buf[:i], "\r"))
		w.buf = append(w.buf[:0], w.buf[i+1:]...)
		if line != "" && w.app.ctx != nil {
			rt.EventsEmit(w.app.ctx, "dms:log", line)
		}
	}
	return len(p), nil
}

// fileCache is the ffprobe result cache. It mirrors dms's own implementation: a
// size-bounded RR cache that persists to a JSON file (the -fFprobeCachePath flag)
// so probe results survive restarts. Get/Set satisfy dmslib.Cache.
type fileCache struct {
	mu sync.Mutex
	c  *rrcache.RRCache
}

func newFileCache() *fileCache {
	return &fileCache{c: rrcache.New(64 << 20)}
}

func (fc *fileCache) Get(key interface{}) (interface{}, bool) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	return fc.c.Get(key)
}

func (fc *fileCache) Set(key, value interface{}) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	var size int64
	for _, v := range []interface{}{key, value} {
		if b, err := json.Marshal(v); err == nil {
			size += int64(len(b))
		}
	}
	fc.c.Set(key, value, size)
}

// load reads cached items previously written by save.
func (fc *fileCache) load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var items []dmslib.FfprobeCacheItem
	if err := json.NewDecoder(f).Decode(&items); err != nil {
		return err
	}
	for _, it := range items {
		fc.Set(it.Key, it.Value)
	}
	return nil
}

// save atomically writes the current cache contents to path.
func (fc *fileCache) save(path string) error {
	fc.mu.Lock()
	items := fc.c.Items()
	fc.mu.Unlock()

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	if err := json.NewEncoder(tmp).Encode(items); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// pickInterfaces replicates dms's interface selection: a named interface, or
// all interfaces that are up with a positive MTU.
func pickInterfaces(name string) []net.Interface {
	var ifs []net.Interface
	if name == "" {
		ifs, _ = net.Interfaces()
	} else if i, err := net.InterfaceByName(name); err == nil && i != nil {
		ifs = append(ifs, *i)
	}

	out := make([]net.Interface, 0, len(ifs))
	for _, i := range ifs {
		if i.Flags&net.FlagUp == 0 || i.MTU <= 0 {
			continue
		}
		out = append(out, i)
	}
	return out
}

// makeIpNets parses a comma-separated allow list. Blank means allow everything,
// matching dms's own default behaviour.
func makeIpNets(s string) []*net.IPNet {
	var nets []*net.IPNet
	if len(s) < 1 {
		_, v4, _ := net.ParseCIDR("0.0.0.0/0")
		_, v6, _ := net.ParseCIDR("::/0")
		return append(nets, v4, v6)
	}
	for _, el := range bytes.Split([]byte(s), []byte(",")) {
		token := string(bytes.TrimSpace(el))
		if token == "" {
			continue
		}
		if ip := net.ParseIP(token); ip != nil {
			// single host: /32 for IPv4, /128 for IPv6.
			suffix := "/32"
			if ip.To4() == nil {
				suffix = "/128"
			}
			if _, ipnet, err := net.ParseCIDR(token + suffix); err == nil {
				nets = append(nets, ipnet)
			}
		} else if _, ipnet, err := net.ParseCIDR(token); err == nil {
			nets = append(nets, ipnet)
		}
	}
	return nets
}

// ipFilterListener rejects connections whose remote IP is not in the allow
// list, before they reach any HTTP handler. Each connection is logged.
type ipFilterListener struct {
	net.Listener
	nets     []*net.IPNet
	allowAll bool
	logf     func(string, ...interface{})
}

func (l *ipFilterListener) Accept() (net.Conn, error) {
	for {
		c, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		host, _, _ := net.SplitHostPort(c.RemoteAddr().String())
		if i := strings.IndexByte(host, '%'); i >= 0 { // strip IPv6 zone
			host = host[:i]
		}
		if l.allowAll || ipInNets(net.ParseIP(host), l.nets) {
			l.logf("access ALLOW %s", host)
			return c, nil
		}
		l.logf("access DENY  %s", host)
		_ = c.Close()
	}
}

func ipInNets(ip net.IP, nets []*net.IPNet) bool {
	if ip == nil {
		return false
	}
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// buildIcons loads a PNG file and wraps it as a single dms device icon. The
// width/height are read from the PNG header so the UPnP IconList advertises the
// real dimensions.
func buildIcons(path string) ([]dmslib.Icon, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("not a valid PNG: %w", err)
	}
	return []dmslib.Icon{{
		Width:    cfg.Width,
		Height:   cfg.Height,
		Depth:    8,
		Mimetype: "image/png",
		Bytes:    data,
	}}, nil
}

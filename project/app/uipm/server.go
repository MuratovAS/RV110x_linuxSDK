package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// version is set at build time via ldflags (see Makefile).
var version = "dev"

type Metrics struct {
	CPU    int    `json:"cpu"`
	RAM    int    `json:"ram"`
	Uptime string `json:"uptime"`
}

type cpuStat struct {
	user    uint64
	nice    uint64
	system  uint64
	idle    uint64
	iowait  uint64
	irq     uint64
	softirq uint64
	steal   uint64
}

type netSample struct {
	rx uint64
	tx uint64
	t  time.Time
}

type NetStats struct {
	RX float64 `json:"rx"`
	TX float64 `json:"tx"`
}

type InterfaceInfo struct {
	IPv4 []string `json:"ipv4"`
	IPv6 []string `json:"ipv6"`
	MAC  string   `json:"mac"`
}

var (
	prevCPU cpuStat
	mu      sync.Mutex

	prevNet netSample
	netMu   sync.Mutex
)

func readCPUStat() (cpuStat, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return cpuStat{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			break
		}
		var s cpuStat
		s.user, _ = strconv.ParseUint(fields[1], 10, 64)
		s.nice, _ = strconv.ParseUint(fields[2], 10, 64)
		s.system, _ = strconv.ParseUint(fields[3], 10, 64)
		s.idle, _ = strconv.ParseUint(fields[4], 10, 64)
		s.iowait, _ = strconv.ParseUint(fields[5], 10, 64)
		s.irq, _ = strconv.ParseUint(fields[6], 10, 64)
		s.softirq, _ = strconv.ParseUint(fields[7], 10, 64)
		s.steal, _ = strconv.ParseUint(fields[8], 10, 64)
		return s, nil
	}
	return cpuStat{}, fmt.Errorf("cpu line not found in /proc/stat")
}

func calcMetrics(prev, curr cpuStat) (cpuPct, ioPct int) {
	prevIdle := prev.idle + prev.iowait
	currIdle := curr.idle + curr.iowait
	prevTotal := prev.user + prev.nice + prev.system + prevIdle + prev.irq + prev.softirq + prev.steal
	currTotal := curr.user + curr.nice + curr.system + currIdle + curr.irq + curr.softirq + curr.steal
	total := currTotal - prevTotal
	if total == 0 {
		return 0, 0
	}
	idle := currIdle - prevIdle
	iowait := curr.iowait - prev.iowait
	cpuPct = int((total - idle) * 100 / total)
	ioPct = int(iowait * 100 / total)
	return
}

func readRAM() int {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()

	var total, available uint64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.ParseUint(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			total = val
		case "MemAvailable:":
			available = val
		}
	}
	if total == 0 {
		return 0
	}
	return int((total - available) * 100 / total)
}

func readUptime() string {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return "unknown"
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return "unknown"
	}
	secs, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return "unknown"
	}
	total := int(secs)
	days := total / 86400
	hours := (total % 86400) / 3600
	mins := (total % 3600) / 60
	return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
}

func readNetBytes() (rx, tx uint64, err error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Scan() // skip header line 1
	scanner.Scan() // skip header line 2
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		iface := strings.TrimSpace(parts[0])
		if iface == "lo" {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 9 {
			continue
		}
		rxB, _ := strconv.ParseUint(fields[0], 10, 64)
		txB, _ := strconv.ParseUint(fields[8], 10, 64)
		rx += rxB
		tx += txB
	}
	return rx, tx, nil
}

type WifiNetwork struct {
	SSID     string `json:"ssid"`
	Signal   int    `json:"signal"`
	Security string `json:"security"`
}

func findWirelessIface() string {
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if _, err := os.Stat("/sys/class/net/" + iface.Name + "/wireless"); err == nil {
			return iface.Name
		}
	}
	return ""
}

func parseIwlistOutput(data string) []WifiNetwork {
	var networks []WifiNetwork
	var cur *WifiNetwork

	for _, line := range strings.Split(data, "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(trimmed, "Cell ") && strings.Contains(trimmed, "Address:") {
			if cur != nil && cur.SSID != "" {
				networks = append(networks, *cur)
			}
			cur = &WifiNetwork{Security: "open", Signal: -100}
			continue
		}
		if cur == nil {
			continue
		}

		if strings.HasPrefix(trimmed, "ESSID:") {
			ssid := strings.TrimPrefix(trimmed, "ESSID:")
			cur.SSID = strings.Trim(ssid, `"`)
		}

		if idx := strings.Index(trimmed, "Signal level="); idx >= 0 {
			rest := trimmed[idx+len("Signal level="):]
			if fields := strings.Fields(rest); len(fields) > 0 {
				if val, err := strconv.Atoi(fields[0]); err == nil {
					cur.Signal = val
				}
			}
		}

		if strings.Contains(trimmed, "IEEE 802.11i/WPA2") || strings.Contains(trimmed, "WPA2") {
			cur.Security = "wpa2"
		} else if strings.Contains(trimmed, "WPA Version") && cur.Security != "wpa2" {
			cur.Security = "wpa"
		}
	}
	if cur != nil && cur.SSID != "" {
		networks = append(networks, *cur)
	}

	seen := make(map[string]WifiNetwork)
	for _, n := range networks {
		if existing, ok := seen[n.SSID]; !ok || n.Signal > existing.Signal {
			seen[n.SSID] = n
		}
	}
	result := make([]WifiNetwork, 0, len(seen))
	for _, n := range seen {
		result = append(result, n)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Signal > result[j].Signal
	})
	return result
}

type UsbipDevice struct {
	BusID     string `json:"busid"`
	VendorID  string `json:"vendorId"`
	ProductID string `json:"productId"`
	Name      string `json:"name"`
	Port      int    `json:"port"`
	Occupied  bool   `json:"occupied"`
}

func parseUsbipList(data string) []UsbipDevice {
	var devices []UsbipDevice
	var cur *UsbipDevice
	for _, line := range strings.Split(data, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- busid ") {
			if cur != nil {
				devices = append(devices, *cur)
			}
			parts := strings.Fields(trimmed)
			if len(parts) < 4 {
				cur = nil
				continue
			}
			busid := parts[2]
			vidpid := strings.Trim(parts[3], "()")
			vidparts := strings.SplitN(vidpid, ":", 2)
			vendorID, productID := "", ""
			if len(vidparts) == 2 {
				vendorID = vidparts[0]
				productID = vidparts[1]
			}
			// busid format: "<bus>-<root>.<port1>.<port2>..."
			// Physical managed port is always chain[1] regardless of hub depth.
			// Examples:
			//   "3-1.1"       → chain=[1,1]       → port 1
			//   "3-1.4"       → chain=[1,4]       → port 4
			//   "3-1.3.3"     → chain=[1,3,3]     → port 3
			//   "3-1.3.4"     → chain=[1,3,4]     → port 3
			//   "3-1.3.3.2.1" → chain=[1,3,3,2,1] → port 3
			port := 0
			if dashIdx := strings.Index(busid, "-"); dashIdx >= 0 {
				chain := strings.Split(busid[dashIdx+1:], ".")
				switch {
				case len(chain) >= 2:
					port, _ = strconv.Atoi(chain[1])
				case len(chain) == 1:
					port, _ = strconv.Atoi(chain[0])
				}
			}
			cur = &UsbipDevice{
				BusID:     busid,
				VendorID:  vendorID,
				ProductID: productID,
				Port:      port,
			}
		} else if cur != nil && trimmed != "" && cur.Name == "" {
			name := trimmed
			if idx := strings.LastIndex(name, " ("); idx >= 0 {
				name = strings.TrimSpace(name[:idx])
			}
			cur.Name = name
		}
	}
	if cur != nil {
		devices = append(devices, *cur)
	}
	return devices
}

// usbNameCache stores busid→name pairs obtained from "usbip list -l".
// It is only refreshed when a new busid appears, reducing expensive calls.
var (
	usbNameCache   = map[string]string{}
	usbNameCacheMu sync.Mutex
)

// scanSysfsDevices reads USB device info directly from sysfs (cheap).
// Entries starting with "usb" are root hubs; entries containing ":" are
// interface nodes — both are skipped.
func scanSysfsDevices() []UsbipDevice {
	var result []UsbipDevice
	entries, err := os.ReadDir("/sys/bus/usb/devices")
	if err != nil {
		return result
	}
	for _, e := range entries {
		busid := e.Name()
		if strings.HasPrefix(busid, "usb") || strings.ContainsRune(busid, ':') {
			continue
		}
		// Skip USB hubs (bDeviceClass == 09).
		if cls, err := os.ReadFile(fmt.Sprintf("/sys/bus/usb/devices/%s/bDeviceClass", busid)); err == nil {
			if strings.TrimSpace(string(cls)) == "09" {
				continue
			}
		}
		vendorData, err := os.ReadFile(fmt.Sprintf("/sys/bus/usb/devices/%s/idVendor", busid))
		if err != nil {
			continue
		}
		dev := UsbipDevice{
			BusID:    busid,
			VendorID: strings.TrimSpace(string(vendorData)),
		}
		if d, err := os.ReadFile(fmt.Sprintf("/sys/bus/usb/devices/%s/idProduct", busid)); err == nil {
			dev.ProductID = strings.TrimSpace(string(d))
		}
		if d, err := os.ReadFile(fmt.Sprintf("/sys/bus/usb/devices/%s/usbip_status", busid)); err == nil {
			dev.Occupied = strings.TrimSpace(string(d)) == "2"
		}
		if dashIdx := strings.Index(busid, "-"); dashIdx >= 0 {
			chain := strings.Split(busid[dashIdx+1:], ".")
			switch {
			case len(chain) >= 2:
				dev.Port, _ = strconv.Atoi(chain[1])
			case len(chain) == 1:
				dev.Port, _ = strconv.Atoi(chain[0])
			}
		}
		result = append(result, dev)
	}
	return result
}

// ensureNamesFor calls "usbip list -l" only when busids contains entries
// not yet present in usbNameCache, then updates the cache.
func ensureNamesFor(busids []string) {
	usbNameCacheMu.Lock()
	defer usbNameCacheMu.Unlock()

	needFetch := false
	for _, id := range busids {
		if _, ok := usbNameCache[id]; !ok {
			needFetch = true
			break
		}
	}
	if !needFetch {
		return
	}

	out, err := runCmdOutput("usbip", "list", "-l")
	if err == nil {
		for _, d := range parseUsbipList(string(out)) {
			usbNameCache[d.BusID] = d.Name
		}
	}
	// Seed missing entries with "" so we don't retry on every request.
	for _, id := range busids {
		if _, ok := usbNameCache[id]; !ok {
			usbNameCache[id] = ""
		}
	}
}

func usbDevicesHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	devices := scanSysfsDevices()

	busids := make([]string, len(devices))
	for i, d := range devices {
		busids[i] = d.BusID
	}
	ensureNamesFor(busids)

	usbNameCacheMu.Lock()
	current := make(map[string]bool, len(devices))
	for i := range devices {
		devices[i].Name = usbNameCache[devices[i].BusID]
		current[devices[i].BusID] = true
	}
	for id := range usbNameCache {
		if !current[id] {
			delete(usbNameCache, id)
		}
	}
	usbNameCacheMu.Unlock()

	if devices == nil {
		devices = []UsbipDevice{}
	}
	json.NewEncoder(w).Encode(devices)
}

func wifiScanHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	iface := findWirelessIface()
	if iface == "" {
		json.NewEncoder(w).Encode([]WifiNetwork{})
		return
	}

	_ = runCmd("iwconfig", iface, "power", "on")
	time.Sleep(300 * time.Millisecond)

	out, err := runCmdOutput("iwlist", iface, "scanning")
	if err != nil {
		json.NewEncoder(w).Encode([]WifiNetwork{})
		return
	}
	json.NewEncoder(w).Encode(parseIwlistOutput(string(out)))
}

func interfacesHandler(w http.ResponseWriter, _ *http.Request) {
	ifaces, err := net.Interfaces()
	if err != nil {
		http.Error(w, "failed to list interfaces", http.StatusInternalServerError)
		return
	}

	result := make(map[string]InterfaceInfo)
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		info := InterfaceInfo{IPv4: []string{}, IPv6: []string{}, MAC: iface.HardwareAddr.String()}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil {
				continue
			}
			if ip.To4() != nil {
				info.IPv4 = append(info.IPv4, ip.String())
			} else {
				info.IPv6 = append(info.IPv6, ip.String())
			}
		}
		result[iface.Name] = info
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func networkHandler(w http.ResponseWriter, r *http.Request) {
	netMu.Lock()
	defer netMu.Unlock()

	rx, tx, err := readNetBytes()
	if err != nil {
		http.Error(w, "failed to read /proc/net/dev", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	elapsed := now.Sub(prevNet.t).Seconds()

	var rxMB, txMB float64
	if elapsed > 0 && !prevNet.t.IsZero() {
		rxMB = float64(rx-prevNet.rx) / elapsed / 1024 / 1024
		txMB = float64(tx-prevNet.tx) / elapsed / 1024 / 1024
	}

	prevNet = netSample{rx: rx, tx: tx, t: now}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(NetStats{RX: rxMB, TX: txMB})
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	curr, err := readCPUStat()
	if err != nil {
		http.Error(w, "failed to read /proc/stat", http.StatusInternalServerError)
		return
	}

	cpu, _ := calcMetrics(prevCPU, curr)
	prevCPU = curr

	m := Metrics{
		CPU:    cpu,
		RAM:    readRAM(),
		Uptime: readUptime(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m)
}

func main() {
	addr := flag.String("addr", ":8080", "address to listen on")
	dir := flag.String("dir", "./dist", "directory to serve")
	config := flag.String("config", "config.json", "path to config file")
	flag.Parse()

	cfgPath = *config
	if err := loadConfig(cfgPath); err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	applyPorts(nil, cfg.Ports)

	var err error
	prevCPU, err = readCPUStat()
	if err != nil {
		log.Fatalf("failed to read initial CPU stat: %v", err)
	}

	prevNet.rx, prevNet.tx, _ = readNetBytes()
	prevNet.t = time.Now()

	http.HandleFunc("/api/config", configHandler)
	http.HandleFunc("/api/metrics", metricsHandler)
	http.HandleFunc("/api/network", networkHandler)
	http.HandleFunc("/api/interfaces", interfacesHandler)
	http.HandleFunc("/api/wifi/scan", wifiScanHandler)
	http.HandleFunc("/api/usb/devices", usbDevicesHandler)
	http.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"version": version})
	})
	http.HandleFunc("/api/errors", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		errs := drainCmdErrors()
		if errs == nil {
			errs = []cmdError{}
		}
		json.NewEncoder(w).Encode(errs)
	})

	fs := http.FileServer(http.Dir(*dir))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fs.ServeHTTP(w, r)
	})

	log.Printf("Serving %s on http://localhost%s", *dir, *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal(err)
	}
}

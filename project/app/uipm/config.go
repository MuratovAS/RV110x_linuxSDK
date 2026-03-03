package main

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// FirewallRule defines which interfaces are allowed for a given service.
type FirewallRule struct {
	Eth  bool `json:"eth"`
	Wifi bool `json:"wifi"`
	VPN  bool `json:"vpn"`
}

// FirewallCfg holds the 3×3 app×interface permission matrix.
type FirewallCfg struct {
	USBIP FirewallRule `json:"usbip"`
	SSH   FirewallRule `json:"ssh"`
	Web   FirewallRule `json:"web"`
}

// SystemCfg holds system-level settings persisted to config.json.
type SystemCfg struct {
	Hostname     string `json:"hostname,omitempty"`
	PasswordHash string `json:"passwordHash,omitempty"`
	PasswordSalt string `json:"passwordSalt,omitempty"`
}

// systemPutReq is the transient shape sent by the frontend for system settings.
// It is never stored to disk.
type systemPutReq struct {
	Hostname      string `json:"hostname"`
	Password      string `json:"password"`
	ClearPassword bool   `json:"clearPassword"`
}

// configPutReq mirrors AppConfig but uses systemPutReq for the system field.
type configPutReq struct {
	Ports     []PortState  `json:"ports"`
	Ethernet  EthernetCfg  `json:"ethernet"`
	WiFi      WiFiCfg      `json:"wifi"`
	WireGuard WireGuardCfg `json:"wireguard"`
	Tailscale TailscaleCfg `json:"tailscale"`
	SSH       SSHCfg       `json:"ssh"`
	System    systemPutReq `json:"system"`
	Firewall  FirewallCfg  `json:"firewall"`
}

type PortState struct {
	ID    int    `json:"id"`
	Power bool   `json:"power"`
	Name  string `json:"name,omitempty"`
}

type EthernetCfg struct {
	Blocked bool   `json:"blocked"`
	Mode    string `json:"mode"`
	IP      string `json:"ip,omitempty"`
	Mask    string `json:"mask,omitempty"`
	Gateway string `json:"gateway,omitempty"`
	DNS     string `json:"dns,omitempty"`
}

type WiFiCfg struct {
	Blocked  bool   `json:"blocked"`
	Enabled  bool   `json:"enabled"`
	SSID     string `json:"ssid,omitempty"`
	Password string `json:"password,omitempty"`
	IP       string `json:"ip,omitempty"`
	Security string `json:"security,omitempty"`
}

type WireGuardCfg struct {
	Blocked bool   `json:"blocked"`
	Enabled bool   `json:"enabled"`
	Config  string `json:"config,omitempty"`
}

type TailscaleCfg struct {
	Blocked    bool   `json:"blocked"`
	Enabled    bool   `json:"enabled"`
	PreAuthKey string `json:"preauthkey,omitempty"`
	ExitNode   bool   `json:"exitNode"`
	ServerURL  string `json:"serverUrl,omitempty"`
}

type SSHKey struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	PublicKey string `json:"publicKey"`
}

type SSHCfg struct {
	Keys []SSHKey `json:"keys"`
}

type AppConfig struct {
	Ports     []PortState  `json:"ports"`
	Ethernet  EthernetCfg  `json:"ethernet"`
	WiFi      WiFiCfg      `json:"wifi"`
	WireGuard WireGuardCfg `json:"wireguard"`
	Tailscale TailscaleCfg `json:"tailscale"`
	SSH       SSHCfg       `json:"ssh"`
	System    SystemCfg    `json:"system"`
	Firewall  FirewallCfg  `json:"firewall"`
}

func defaultConfig() AppConfig {
	allAllowed := FirewallRule{Eth: true, Wifi: true, VPN: true}
	return AppConfig{
		Ports: []PortState{
			{ID: 1, Power: true},
			{ID: 2, Power: true},
			{ID: 3, Power: true},
			{ID: 4, Power: true},
		},
		Ethernet:  EthernetCfg{Mode: "dhcp", IP: "192.168.1.100", Gateway: "192.168.1.1"},
		WiFi:      WiFiCfg{Blocked: false, Enabled: false},
		WireGuard: WireGuardCfg{Blocked: false, Enabled: false, Config: "[Interface]\nPrivateKey = ...\nAddress = 10.0.0.5/32"},
		Tailscale: TailscaleCfg{Enabled: false, ServerURL: "https://controlplane.tailscale.com"},
		Firewall:  FirewallCfg{USBIP: allAllowed, SSH: allAllowed, Web: allAllowed},
	}
}

var (
	cfg     AppConfig
	cfgPath string
	cfgMu   sync.RWMutex
)

func loadConfig(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg = defaultConfig()
		seedHostname()
		return writeConfig(path)
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	seedHostname()
	return nil
}

// seedHostname reads /etc/hostname when no hostname is stored in config yet.
func seedHostname() {
	if cfg.System.Hostname != "" {
		return
	}
	if data, err := os.ReadFile("/etc/hostname"); err == nil {
		cfg.System.Hostname = strings.TrimSpace(string(data))
	}
}

// applyHostname writes /etc/hostname and runs the hostname(1) command.
func applyHostname(hostname string) {
	if hostname == "" {
		return
	}
	if err := os.WriteFile("/etc/hostname", []byte(hostname+"\n"), 0644); err != nil {
		pushCmdError("system: write hostname: " + err.Error())
		return
	}
	runCmd("hostname", hostname) //nolint:errcheck
}

func writeConfig(path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

const wgConfPath = "/etc/wireguard/wg0.conf"
const networkInterfacesPath = "/etc/network/interfaces"

func findEthernetIface() string {
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if _, err := os.Stat("/sys/class/net/" + iface.Name + "/wireless"); err == nil {
			continue
		}
		return iface.Name
	}
	return "eth0"
}

func applyNetworkInterfaces(eth EthernetCfg, wifi WiFiCfg) {
	ethIface := findEthernetIface()
	wifiIface := findWirelessIface()

	var sb strings.Builder
	sb.WriteString("# Generated by uipm\n")
	sb.WriteString("auto lo\n")
	sb.WriteString("iface lo inet loopback\n")

	if !eth.Blocked {
		sb.WriteString("\nauto " + ethIface + "\n")
		if eth.Mode == "static" {
			sb.WriteString("iface " + ethIface + " inet static\n")
			if eth.IP != "" {
				sb.WriteString("\taddress " + eth.IP + "\n")
			}
			mask := eth.Mask
			if mask == "" {
				mask = "255.255.255.0"
			}
			sb.WriteString("\tnetmask " + mask + "\n")
			if eth.Gateway != "" {
				sb.WriteString("\tgateway " + eth.Gateway + "\n")
			}
			if eth.DNS != "" {
				sb.WriteString("\tdns-nameservers " + eth.DNS + "\n")
			}
		} else {
			sb.WriteString("iface " + ethIface + " inet dhcp\n")
		}
	}

	if !wifi.Blocked && wifi.Enabled && wifiIface != "" {
		sb.WriteString("\nauto " + wifiIface + "\n")
		if wifi.IP != "" {
			sb.WriteString("iface " + wifiIface + " inet static\n")
			sb.WriteString("\taddress " + wifi.IP + "\n")
			sb.WriteString("\tnetmask 255.255.255.0\n")
		} else {
			sb.WriteString("iface " + wifiIface + " inet dhcp\n")
		}
		if wifi.SSID != "" {
			sb.WriteString("\twpa-ssid " + wifi.SSID + "\n")
		}
		if wifi.Security == "wpa2" && wifi.Password != "" {
			sb.WriteString("\twpa-psk " + wifi.Password + "\n")
		}
	}

	if err := os.WriteFile(networkInterfacesPath, []byte(sb.String()), 0644); err != nil {
		pushCmdError("network: write interfaces: " + err.Error())
		return
	}

	runCmd("/etc/init.d/S45ifplugd", "restart") //nolint:errcheck
}

// processWGConfig strips DNS lines and injects Table = off into [Interface].
// It also prepends "#Enabled=<true|false>" as the very first line so that
// the init.d script can skip bringing up the interface when disabled.
func processWGConfig(raw string, enabled bool) string {
	enabledLine := "#Enabled=false"
	if enabled {
		enabledLine = "#Enabled=true"
	}
	var out []string
	out = append(out, enabledLine)
	inInterface := false
	tableAdded := false

	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		// Drop any existing #Enabled= line so it is not duplicated.
		if strings.HasPrefix(trimmed, "#Enabled=") {
			continue
		}

		if strings.HasPrefix(trimmed, "[") {
			if inInterface && !tableAdded {
				out = append(out, "Table = off")
				tableAdded = true
			}
			inInterface = strings.EqualFold(trimmed, "[Interface]")
			tableAdded = false
		}

		// Drop DNS lines in any section
		if strings.HasPrefix(strings.ToLower(trimmed), "dns") && strings.Contains(trimmed, "=") {
			continue
		}

		// Inject Table = off right after Address line inside [Interface]
		out = append(out, line)
		if inInterface && !tableAdded && strings.HasPrefix(strings.ToLower(trimmed), "address") {
			out = append(out, "Table = off")
			tableAdded = true
		}
	}

	// In case file ends while still in [Interface] without Table added
	if inInterface && !tableAdded {
		out = append(out, "Table = off")
	}

	return strings.Join(out, "\n")
}

const authorizedKeysPath = "/root/.ssh/authorized_keys"

func applySSHKeys(ssh SSHCfg) {
	var sb strings.Builder
	for _, key := range ssh.Keys {
		sb.WriteString(strings.TrimSpace(key.PublicKey))
		sb.WriteString("\n")
	}
	if err := os.MkdirAll("/root/.ssh", 0700); err != nil {
		pushCmdError("ssh: mkdir .ssh: " + err.Error())
		return
	}
	if err := os.WriteFile(authorizedKeysPath, []byte(sb.String()), 0600); err != nil {
		pushCmdError("ssh: write authorized_keys: " + err.Error())
	}
}

const (
	iptablesConfPath  = "/etc/iptables.conf"
	ip6tablesConfPath = "/etc/ip6tables.conf"
)

// buildFirewallRules generates an iptables-restore compatible ruleset from fw.
// Default policy: DROP all INPUT; allow loopback, established, and the
// specific app×interface combinations that are enabled in the matrix.
// If extraRules is non-empty, those lines are inserted after the base allow rules.
func buildFirewallRules(fw FirewallCfg, extraRules []string) string {
	ethIface := findEthernetIface()
	wifiIface := findWirelessIface()

	type entry struct {
		rule  FirewallRule
		ports []int
	}
	apps := []entry{
		{fw.USBIP, []int{3240}},
		{fw.SSH, []int{22}},
		{fw.Web, []int{80, 443}},
	}

	var sb strings.Builder
	sb.WriteString("*filter\n")
	sb.WriteString(":INPUT DROP [0:0]\n")
	sb.WriteString(":FORWARD ACCEPT [0:0]\n")
	sb.WriteString(":OUTPUT ACCEPT [0:0]\n")
	sb.WriteString("-A INPUT -i lo -j ACCEPT\n")
	sb.WriteString("-A INPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT\n")
	for _, r := range extraRules {
		sb.WriteString(r + "\n")
	}

	addRules := func(iface string, ports []int) {
		for _, port := range ports {
			sb.WriteString("-A INPUT -i " + iface + " -p tcp --dport " + strconv.Itoa(port) + " -j ACCEPT\n")
		}
	}

	for _, app := range apps {
		if app.rule.Eth && ethIface != "" {
			addRules(ethIface, app.ports)
		}
		if app.rule.Wifi && wifiIface != "" {
			addRules(wifiIface, app.ports)
		}
		if app.rule.VPN {
			for _, vpnIface := range []string{"wg0", "tailscale0"} {
				addRules(vpnIface, app.ports)
			}
		}
	}

	sb.WriteString("COMMIT\n")
	return sb.String()
}

// applyFirewall writes iptables.conf + ip6tables.conf and restarts both services.
func applyFirewall(fw FirewallCfg) {
	if err := os.WriteFile(iptablesConfPath, []byte(buildFirewallRules(fw, nil)), 0644); err != nil {
		pushCmdError("firewall: write iptables.conf: " + err.Error())
		return
	}
	runCmd("/etc/init.d/S35iptables", "restart") //nolint:errcheck

	ip6Rules := []string{"-A INPUT -p ipv6-icmp -j ACCEPT"}
	if err := os.WriteFile(ip6tablesConfPath, []byte(buildFirewallRules(fw, ip6Rules)), 0644); err != nil {
		pushCmdError("firewall: write ip6tables.conf: " + err.Error())
		return
	}
	runCmd("/etc/init.d/S36ip6tables", "restart") //nolint:errcheck
}

func saveWGConfig(raw string, enabled bool) error {
	if err := os.MkdirAll("/etc/wireguard", 0700); err != nil {
		return err
	}
	processed := processWGConfig(raw, enabled)
	return os.WriteFile(wgConfPath, []byte(processed), 0600)
}

func applyWireGuard(old, wg WireGuardCfg) {
	configChanged := old.Config != wg.Config
	enabledChanged := old.Enabled != wg.Enabled

	if (configChanged || enabledChanged) && wg.Config != "" {
		if err := saveWGConfig(wg.Config, wg.Enabled); err != nil {
			pushCmdError("wireguard: save config: " + err.Error())
			return
		}
	}

	wasEnabled := old.Enabled
	nowEnabled := wg.Enabled

	switch {
	case wasEnabled && configChanged && nowEnabled:
		// Restart to apply new config
		runCmd("wg-quick", "down", "wg0") //nolint:errcheck
		runCmd("wg-quick", "up", "wg0")   //nolint:errcheck
	case !wasEnabled && nowEnabled:
		runCmd("wg-quick", "up", "wg0") //nolint:errcheck
	case wasEnabled && !nowEnabled:
		runCmd("wg-quick", "down", "wg0") //nolint:errcheck
	}
}

func applyTailscale(old, ts TailscaleCfg) {
	if old.PreAuthKey != ts.PreAuthKey {
		runCmd("tailscale", "logout") //nolint:errcheck
		loginArgs := []string{"login", "--auth-key=" + ts.PreAuthKey}
		if ts.ServerURL != "" {
			loginArgs = append(loginArgs, "--login-server="+ts.ServerURL)
		}
		loginArgs = append(loginArgs, "--advertise-exit-node="+strconv.FormatBool(ts.ExitNode))
		runCmd("tailscale", loginArgs...) //nolint:errcheck
		if !ts.Enabled {
			runCmd("tailscale", "down") //nolint:errcheck
		}
		return
	}

	if ts.Enabled {
		if old.Enabled {
			runCmd("tailscale", "down") //nolint:errcheck
		}
		upArgs := []string{"up"}
		if ts.ServerURL != "" {
			upArgs = append(upArgs, "--login-server="+ts.ServerURL)
		}
		upArgs = append(upArgs, "--advertise-exit-node="+strconv.FormatBool(ts.ExitNode))
		upArgs = append(upArgs, "--timeout=5s")
		runCmd("tailscale", upArgs...) //nolint:errcheck
	}
	if !ts.Enabled && old.Enabled {
		runCmd("tailscale", "down") //nolint:errcheck
	}
}

func applyPorts(oldPorts, newPorts []PortState) {
	oldMap := make(map[int]bool, len(oldPorts))
	for _, p := range oldPorts {
		oldMap[p.ID] = p.Power
	}
	for _, p := range newPorts {
		prev, exists := oldMap[p.ID]
		if !exists || prev != p.Power {
			state := "off"
			if p.Power {
				state = "on"
			}
			runCmd("usbpwr", strconv.Itoa(p.ID), state) //nolint:errcheck
		}
	}
}

func configHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfgMu.RLock()
		out := cfg
		cfgMu.RUnlock()
		if out.WiFi.Password != "" {
			out.WiFi.Password = "***"
		}
		if out.Tailscale.PreAuthKey != "" {
			out.Tailscale.PreAuthKey = "***"
		}
		if out.WireGuard.Config != "" {
			out.WireGuard.Config = "***"
		}
		// Never expose credentials to the client
		out.System.PasswordHash = ""
		out.System.PasswordSalt = ""
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)

	case http.MethodPut:
		var req configPutReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		cfgMu.Lock()
		oldPorts := cfg.Ports
		oldEth := cfg.Ethernet
		oldWifi := cfg.WiFi
		oldTs := cfg.Tailscale
		oldWg := cfg.WireGuard
		oldSystem := cfg.System
		oldFirewall := cfg.Firewall

		// Restore masked sensitive fields
		wifi := req.WiFi
		if wifi.Password == "***" {
			wifi.Password = cfg.WiFi.Password
		}
		ts := req.Tailscale
		if ts.PreAuthKey == "***" {
			ts.PreAuthKey = cfg.Tailscale.PreAuthKey
		}
		wg := req.WireGuard
		if wg.Config == "***" {
			wg.Config = cfg.WireGuard.Config
		}

		// Build system config — keep existing hash/salt unless explicitly changed
		newSystem := SystemCfg{
			Hostname:     req.System.Hostname,
			PasswordHash: cfg.System.PasswordHash,
			PasswordSalt: cfg.System.PasswordSalt,
		}
		if req.System.ClearPassword {
			newSystem.PasswordHash = ""
			newSystem.PasswordSalt = ""
		} else if req.System.Password != "" {
			salt := generateSalt()
			newSystem.PasswordHash = hashPassword(req.System.Password, salt)
			newSystem.PasswordSalt = salt
		}

		newCfg := AppConfig{
			Ports:     req.Ports,
			Ethernet:  req.Ethernet,
			WiFi:      wifi,
			WireGuard: wg,
			Tailscale: ts,
			SSH:       req.SSH,
			System:    newSystem,
			Firewall:  req.Firewall,
		}

		netChanged := oldEth != newCfg.Ethernet || oldWifi != newCfg.WiFi
		tsChanged := oldTs != newCfg.Tailscale
		wgChanged := oldWg != newCfg.WireGuard
		hostnameChanged := oldSystem.Hostname != newSystem.Hostname
		passwordCleared := req.System.ClearPassword
		firewallChanged := oldFirewall != newCfg.Firewall

		cfg = newCfg
		err := writeConfig(cfgPath)
		cfgMu.Unlock()

		if err != nil {
			http.Error(w, "failed to save config", http.StatusInternalServerError)
			return
		}

		go applyPorts(oldPorts, newCfg.Ports)
		if netChanged {
			go applyNetworkInterfaces(newCfg.Ethernet, newCfg.WiFi)
		}
		if tsChanged {
			go applyTailscale(oldTs, newCfg.Tailscale)
		}
		if wgChanged {
			go applyWireGuard(oldWg, newCfg.WireGuard)
		}
		go applySSHKeys(newCfg.SSH)
		if firewallChanged {
			go applyFirewall(newCfg.Firewall)
		}
		if hostnameChanged {
			go applyHostname(newSystem.Hostname)
		}
		// When password is cleared, invalidate all active sessions so that the
		// open-access mode takes effect immediately.
		if passwordCleared {
			sessionsMu.Lock()
			sessions = map[string]time.Time{}
			sessionsMu.Unlock()
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

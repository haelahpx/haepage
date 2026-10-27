// Package system reads host metrics (CPU, memory, disks, network,
// temperature) from /proc and /sys for the server stats tile.
package system

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Stats is the body of GET /api/system. Sizes are bytes, rates bytes per
// second.
type Stats struct {
	Hostname string   `json:"hostname"`
	OS       string   `json:"os"`
	Uptime   int64    `json:"uptime"` // seconds
	CPU      CPU      `json:"cpu"`
	Memory   Usage    `json:"memory"`
	Disks    []Disk   `json:"disks"`
	Network  Network  `json:"network"`
	TempC    *float64 `json:"tempC"`
	Temps    []Sensor `json:"temps"`
}

// Sensor is one temperature device: its hottest reading and, when the
// hardware reports one, its critical limit.
type Sensor struct {
	Name  string   `json:"name"`
	C     float64  `json:"c"`
	CritC *float64 `json:"critC,omitempty"`
}

type CPU struct {
	Percent float64 `json:"percent"`
	Cores   int     `json:"cores"`
	Load1   float64 `json:"load1"`
}

type Usage struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
}

type Disk struct {
	Path  string `json:"path"`
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
}

type Network struct {
	RxRate float64 `json:"rxRate"`
	TxRate float64 `json:"txRate"`
}

// counters are the cumulative values that rates are computed from.
type counters struct {
	at          time.Time
	cpuBusy     uint64
	cpuTotal    uint64
	rxBytes     uint64
	txBytes     uint64
	initialized bool
}

// Sampler reads stats on demand. CPU and network rates need two readings;
// when the last one is stale, Read takes a fresh pair a short moment apart,
// so nothing runs in the background while nobody is looking.
type Sampler struct {
	Root   string   // "/" in production; a fixture folder in tests
	Mounts []string // paths whose filesystems are reported, e.g. "/" and the data folder

	mu   sync.Mutex
	prev counters
	now  func() time.Time
	wait func(time.Duration)
}

const (
	staleAfter = 15 * time.Second
	pairGap    = 250 * time.Millisecond
)

func NewSampler(mounts ...string) *Sampler {
	return &Sampler{Root: "/", Mounts: mounts}
}

func (s *Sampler) path(p string) string { return filepath.Join(s.Root, p) }

// Read returns current stats. Parts the host doesn't expose are left zero.
func (s *Sampler) Read() (Stats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.now == nil {
		s.now = time.Now
	}
	if s.wait == nil {
		s.wait = time.Sleep
	}

	cur, err := s.readCounters()
	if err != nil {
		return Stats{}, err
	}
	if !s.prev.initialized || cur.at.Sub(s.prev.at) > staleAfter {
		s.prev = cur
		s.wait(pairGap)
		if cur, err = s.readCounters(); err != nil {
			return Stats{}, err
		}
	}

	st := Stats{
		OS:     s.osName(),
		Uptime: s.uptime(),
		CPU:    CPU{Cores: runtime.NumCPU(), Load1: s.load1()},
		Memory: s.memory(),
		Disks:  diskUsage(s.Mounts),
		TempC:  s.temperature(),
		Temps:  s.sensors(),
	}
	st.Hostname, _ = os.Hostname()

	if dt := cur.at.Sub(s.prev.at).Seconds(); dt > 0 {
		if total := cur.cpuTotal - s.prev.cpuTotal; total > 0 {
			st.CPU.Percent = round1(float64(cur.cpuBusy-s.prev.cpuBusy) / float64(total) * 100)
		}
		st.Network.RxRate = float64(cur.rxBytes-s.prev.rxBytes) / dt
		st.Network.TxRate = float64(cur.txBytes-s.prev.txBytes) / dt
	}
	s.prev = cur
	return st, nil
}

func (s *Sampler) readCounters() (counters, error) {
	busy, total, err := s.cpuTimes()
	if err != nil {
		return counters{}, err
	}
	rx, tx := s.netBytes()
	return counters{at: s.now(), cpuBusy: busy, cpuTotal: total, rxBytes: rx, txBytes: tx, initialized: true}, nil
}

// cpuTimes reads the aggregate "cpu" line of /proc/stat.
func (s *Sampler) cpuTimes() (busy, total uint64, err error) {
	data, err := os.ReadFile(s.path("proc/stat"))
	if err != nil {
		return 0, 0, err
	}
	line, _, _ := strings.Cut(string(data), "\n")
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, errors.New("unexpected /proc/stat format")
	}
	for i, f := range fields[1:] {
		v, _ := strconv.ParseUint(f, 10, 64)
		total += v
		if i != 3 && i != 4 { // idle and iowait are not busy
			busy += v
		}
	}
	return busy, total, nil
}

// netBytes sums traffic on physical interfaces, skipping loopback and the
// virtual interfaces containers create.
func (s *Sampler) netBytes() (rx, tx uint64) {
	f, err := os.Open(s.path("proc/net/dev"))
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		name, rest, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		if virtualInterface(name) {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) < 9 {
			continue
		}
		r, _ := strconv.ParseUint(fields[0], 10, 64)
		t, _ := strconv.ParseUint(fields[8], 10, 64)
		rx += r
		tx += t
	}
	return rx, tx
}

func virtualInterface(name string) bool {
	if name == "lo" {
		return true
	}
	for _, p := range []string{"docker", "veth", "br-", "virbr", "cni", "flannel", "tailscale", "tun", "wg"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func (s *Sampler) memory() Usage {
	f, err := os.Open(s.path("proc/meminfo"))
	if err != nil {
		return Usage{}
	}
	defer f.Close()
	var total, avail uint64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		v, _ := strconv.ParseUint(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			total = v * 1024
		case "MemAvailable:":
			avail = v * 1024
		}
	}
	if avail > total {
		avail = total
	}
	return Usage{Total: total, Used: total - avail}
}

func (s *Sampler) uptime() int64 {
	data, err := os.ReadFile(s.path("proc/uptime"))
	if err != nil {
		return 0
	}
	first, _, _ := strings.Cut(string(data), " ")
	v, _ := strconv.ParseFloat(first, 64)
	return int64(v)
}

func (s *Sampler) load1() float64 {
	data, err := os.ReadFile(s.path("proc/loadavg"))
	if err != nil {
		return 0
	}
	first, _, _ := strings.Cut(string(data), " ")
	v, _ := strconv.ParseFloat(first, 64)
	return v
}

func (s *Sampler) osName() string {
	data, err := os.ReadFile(s.path("etc/os-release"))
	if err != nil {
		return runtime.GOOS
	}
	for _, line := range strings.Split(string(data), "\n") {
		if v, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
			return strings.Trim(v, `"`)
		}
	}
	return runtime.GOOS
}

// temperature returns the hottest CPU-like thermal zone, or nil if there
// is none.
func (s *Sampler) temperature() *float64 {
	zones, _ := filepath.Glob(s.path("sys/class/thermal/thermal_zone*"))
	var best *float64
	for _, z := range zones {
		kind, _ := os.ReadFile(filepath.Join(z, "type"))
		k := strings.ToLower(strings.TrimSpace(string(kind)))
		if !strings.Contains(k, "cpu") && !strings.Contains(k, "pkg") && !strings.Contains(k, "soc") && !strings.Contains(k, "k10temp") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(z, "temp"))
		if err != nil {
			continue
		}
		milli, err := strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
		if err != nil || milli <= 0 {
			continue
		}
		c := round1(milli / 1000)
		if best == nil || c > *best {
			best = &c
		}
	}
	return best
}

// Friendly names for common hwmon drivers.
var sensorNames = map[string]string{
	"coretemp": "CPU", "k10temp": "CPU", "zenpower": "CPU", "cpu_thermal": "CPU", "soc_thermal": "CPU",
	"acpitz": "Motherboard", "pch_cannonlake": "Chipset", "pch_skylake": "Chipset",
	"nvme": "NVMe drive", "drivetemp": "Drive", "amdgpu": "GPU", "nouveau": "GPU", "radeon": "GPU",
	"spd5118": "Memory", "jc42": "Memory", "mt7921_phy0": "Wi-Fi", "iwlwifi_1": "Wi-Fi",
}

// sensors reads every hwmon device with temperature inputs. Devices with
// the same friendly name are numbered ("Memory 2").
func (s *Sampler) sensors() []Sensor {
	devices, _ := filepath.Glob(s.path("sys/class/hwmon/hwmon*"))
	sort.Strings(devices)
	out := []Sensor{}
	seen := map[string]int{}
	readC := func(path string) (float64, bool) {
		raw, err := os.ReadFile(path)
		if err != nil {
			return 0, false
		}
		milli, err := strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
		return milli / 1000, err == nil && milli > 0 && milli < 200000
	}
	for _, dev := range devices {
		inputs, _ := filepath.Glob(filepath.Join(dev, "temp*_input"))
		if len(inputs) == 0 {
			continue
		}
		var hottest float64
		var crit *float64
		for _, in := range inputs {
			if c, ok := readC(in); ok && c > hottest {
				hottest = c
			}
			base := strings.TrimSuffix(in, "_input")
			for _, limit := range []string{base + "_crit", base + "_max"} {
				if c, ok := readC(limit); ok && (crit == nil || c < *crit) {
					v := round1(c)
					crit = &v
					break
				}
			}
		}
		if hottest == 0 {
			continue
		}
		raw, _ := os.ReadFile(filepath.Join(dev, "name"))
		driver := strings.TrimSpace(string(raw))
		name := sensorNames[driver]
		if name == "" {
			name = driver
		}
		if seen[name]++; seen[name] > 1 {
			name = fmt.Sprintf("%s %d", name, seen[name])
		}
		out = append(out, Sensor{Name: name, C: round1(hottest), CritC: crit})
	}
	return out
}

func round1(v float64) float64 {
	return float64(int64(v*10+0.5)) / 10
}

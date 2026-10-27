package system

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// copyHost copies testdata/host into a temp folder the test can modify.
func copyHost(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.Walk("testdata/host", func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("testdata/host", p)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func write(t *testing.T, root, rel, data string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755)
	if err := os.WriteFile(filepath.Join(root, rel), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

const netDevLater = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 9000      10    0    0    0     0          0         0     9000      10    0    0    0     0       0          0
  eth0: 1500000   100   0    0    0     0          0         0   300000     50    0    0    0     0       0          0
docker0: 999999   10    0    0    0     0          0         0   999999     10    0    0    0     0       0          0
`

func TestRead(t *testing.T) {
	root := copyHost(t)
	clock := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	waits := 0
	s := &Sampler{
		Root:   root,
		Mounts: []string{root, root},
		now:    func() time.Time { return clock },
		wait: func(time.Duration) {
			waits++
			clock = clock.Add(time.Second)
			write(t, root, "proc/stat", "cpu  200 0 200 750 150 0 0 0 0 0\n")
			write(t, root, "proc/net/dev", netDevLater)
		},
	}

	st, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	if waits != 1 {
		t.Errorf("first read took %d sample pairs, want 1", waits)
	}
	if st.CPU.Percent != 66.7 {
		t.Errorf("cpu = %v, want 66.7", st.CPU.Percent)
	}
	if st.CPU.Load1 != 0.42 || st.CPU.Cores < 1 {
		t.Errorf("cpu = %+v", st.CPU)
	}
	if st.Memory.Total != 16000000*1024 || st.Memory.Used != 10000000*1024 {
		t.Errorf("memory = %+v", st.Memory)
	}
	// Loopback and docker0 are ignored; eth0 moved 500 KB down, 100 KB up.
	if st.Network.RxRate != 500000 || st.Network.TxRate != 100000 {
		t.Errorf("network = %+v", st.Network)
	}
	if st.TempC == nil || *st.TempC != 51.5 {
		t.Errorf("temp = %v, want 51.5 (the acpitz zone is not a CPU zone)", st.TempC)
	}
	if st.OS != "Debian GNU/Linux 12 (bookworm)" || st.Uptime != 12345 {
		t.Errorf("os = %q uptime = %d", st.OS, st.Uptime)
	}
	if len(st.Disks) != 1 || st.Disks[0].Total == 0 {
		t.Errorf("disks = %+v, want one entry for two paths on the same filesystem", st.Disks)
	}

	// A second read soon after reuses the last sample instead of waiting.
	clock = clock.Add(5 * time.Second)
	if _, err := s.Read(); err != nil {
		t.Fatal(err)
	}
	if waits != 1 {
		t.Errorf("fresh read waited again (%d)", waits)
	}
}

func TestMissingFilesAreTolerated(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "proc"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, root, "proc/stat", "cpu  1 0 1 10 0 0 0 0 0 0\n")
	s := &Sampler{Root: root, wait: func(time.Duration) {}}
	st, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	if st.TempC != nil || st.Memory.Total != 0 || len(st.Disks) != 0 {
		t.Errorf("stats = %+v", st)
	}
}

func TestBadProcStat(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "proc"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, root, "proc/stat", "garbage\n")
	if _, err := (&Sampler{Root: root, wait: func(time.Duration) {}}).Read(); err == nil {
		t.Error("want an error for an unreadable /proc/stat")
	}
}

func TestSensors(t *testing.T) {
	root := t.TempDir()
	write(t, root, "sys/class/hwmon/hwmon0/name", "ACAD\n") // no temperatures
	write(t, root, "sys/class/hwmon/hwmon1/name", "coretemp\n")
	write(t, root, "sys/class/hwmon/hwmon1/temp1_input", "61000\n")
	write(t, root, "sys/class/hwmon/hwmon1/temp1_crit", "100000\n")
	write(t, root, "sys/class/hwmon/hwmon1/temp2_input", "67500\n")
	write(t, root, "sys/class/hwmon/hwmon1/temp2_crit", "100000\n")
	write(t, root, "sys/class/hwmon/hwmon2/name", "nvme\n")
	write(t, root, "sys/class/hwmon/hwmon2/temp1_input", "44850\n")
	write(t, root, "sys/class/hwmon/hwmon2/temp1_max", "84850\n")
	write(t, root, "sys/class/hwmon/hwmon3/name", "spd5118\n")
	write(t, root, "sys/class/hwmon/hwmon3/temp1_input", "40000\n")
	write(t, root, "sys/class/hwmon/hwmon4/name", "spd5118\n")
	write(t, root, "sys/class/hwmon/hwmon4/temp1_input", "41250\n")
	write(t, root, "sys/class/hwmon/hwmon5/name", "weird_sensor\n")
	write(t, root, "sys/class/hwmon/hwmon5/temp1_input", "-5\n") // nonsense values are skipped
	write(t, root, "sys/class/hwmon/hwmon6/name", "fancy_chip\n")
	write(t, root, "sys/class/hwmon/hwmon6/temp1_input", "30000\n")

	got := (&Sampler{Root: root}).sensors()
	var desc []string
	for _, s := range got {
		d := fmt.Sprintf("%s=%.1f", s.Name, s.C)
		if s.CritC != nil {
			d += fmt.Sprintf("/%.1f", *s.CritC)
		}
		desc = append(desc, d)
	}
	want := "CPU=67.5/100.0 NVMe drive=44.9/84.9 Memory=40.0 Memory 2=41.3 fancy_chip=30.0"
	if strings.Join(desc, " ") != want {
		t.Errorf("sensors = %s\nwant      %s", strings.Join(desc, " "), want)
	}
	if empty := (&Sampler{Root: t.TempDir()}).sensors(); empty == nil || len(empty) != 0 {
		t.Errorf("no hwmon = %#v, want an empty list", empty)
	}
}

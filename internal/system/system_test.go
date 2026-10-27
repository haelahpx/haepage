package system

import (
	"os"
	"path/filepath"
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

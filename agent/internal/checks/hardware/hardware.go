package hardware

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/u2yasan/ssnp_sip/agent/internal/policy"
)

type Result struct {
	CPUCheckPassed         bool
	RAMCheckPassed         bool
	StorageSizeCheckPassed bool
	SSDCheckPassed         bool
	VisibleCPUThreads      int
	VisibleMemoryBytes     uint64
	VisibleStorageBytes    uint64
}

func Run(tempDir string, thresholds policy.HardwareThresholds) Result {
	threads := runtime.NumCPU()
	memory := visibleMemory()
	storage := visibleStorage(tempDir)
	ssd := detectSSD(tempDir)

	return Result{
		CPUCheckPassed:         threads >= thresholds.CPUCoresMin,
		RAMCheckPassed:         bytesToGiB(memory) >= uint64(thresholds.RAMGBMin),
		StorageSizeCheckPassed: bytesToGiB(storage) >= uint64(thresholds.StorageGBMin),
		SSDCheckPassed:         !thresholds.SSDRequired || ssd,
		VisibleCPUThreads:      threads,
		VisibleMemoryBytes:     memory,
		VisibleStorageBytes:    storage,
	}
}

func visibleMemory() uint64 {
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/proc/meminfo")
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "MemTotal:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						if kb, convErr := strconv.ParseUint(fields[1], 10, 64); convErr == nil {
							return kb * 1024
						}
					}
				}
			}
		}
	}

	if runtime.GOOS == "darwin" {
		out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
		if err == nil {
			if bytes, convErr := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64); convErr == nil {
				return bytes
			}
		}
	}

	return 0
}

func visibleStorage(path string) uint64 {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0
	}
	return st.Blocks * uint64(st.Bsize)
}

func bytesToGiB(v uint64) uint64 {
	return v / (1024 * 1024 * 1024)
}

func detectSSD(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	cmd := exec.Command("lsblk", "-J", "-o", "ROTA,MOUNTPOINT")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	var payload struct {
		Blockdevices []blockDevice `json:"blockdevices"`
	}
	if err := jsonUnmarshal(out, &payload); err != nil {
		return false
	}
	for _, device := range payload.Blockdevices {
		if rota, ok := matchMount(device, abs); ok {
			return !rota
		}
	}
	return false
}

type blockDevice struct {
	Rota       bool          `json:"rota"`
	Mountpoint string        `json:"mountpoint"`
	Children   []blockDevice `json:"children"`
}

func matchMount(device blockDevice, path string) (bool, bool) {
	if device.Mountpoint != "" && strings.HasPrefix(path, device.Mountpoint) {
		return device.Rota, true
	}
	for _, child := range device.Children {
		if rota, ok := matchMount(child, path); ok {
			return rota, true
		}
	}
	return false, false
}

func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

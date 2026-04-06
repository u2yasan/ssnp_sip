package disk

import (
	"context"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/u2yasan/ssnp_sip/agent/internal/policy"
)

type Result struct {
	Passed             bool
	MeasuredIOPS       float64
	MeasuredLatencyP95 float64
}

func Run(ctx context.Context, tempDir string, profile policy.DiskProfile) Result {
	tempFile, err := os.CreateTemp(tempDir, "ssnp-disk-check-*")
	if err != nil {
		return Result{}
	}
	path := tempFile.Name()
	defer os.Remove(path)
	defer tempFile.Close()

	size := int64(profile.BlockSizeBytes * profile.QueueDepth * 256)
	if err := tempFile.Truncate(size); err != nil {
		return Result{}
	}
	file, err := os.OpenFile(filepath.Clean(path), os.O_RDWR, 0o600)
	if err != nil {
		return Result{}
	}
	defer file.Close()

	warmup := time.Duration(profile.WarmupSeconds) * time.Second
	measured := time.Duration(profile.MeasuredSeconds) * time.Second
	cooldown := time.Duration(profile.CooldownSeconds) * time.Second

	var ops atomic.Uint64
	stop := make(chan struct{})
	startWorkers(file, profile, stop, &ops)
	sleepOrCancel(ctx, warmup)
	ops.Store(0)
	sleepOrCancel(ctx, measured)
	total := ops.Load()
	sleepOrCancel(ctx, cooldown)
	close(stop)

	iops := float64(total) / float64(max(profile.MeasuredSeconds, 1))
	return Result{
		Passed:             iops >= profile.AcceptanceFloor.Minimum,
		MeasuredIOPS:       iops,
		MeasuredLatencyP95: 0,
	}
}

func startWorkers(file *os.File, profile policy.DiskProfile, stop <-chan struct{}, ops *atomic.Uint64) {
	var wg sync.WaitGroup
	for i := 0; i < profile.Concurrency; i++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			block := make([]byte, profile.BlockSizeBytes)
			rng := rand.New(rand.NewSource(seed))
			fileSize := int64(profile.BlockSizeBytes * profile.QueueDepth * 256)
			for {
				select {
				case <-stop:
					return
				default:
					offset := int64(rng.Intn(int(fileSize/int64(profile.BlockSizeBytes)))) * int64(profile.BlockSizeBytes)
					if rng.Float64() < profile.ReadRatio {
						_, _ = file.ReadAt(block, offset)
					} else {
						_, _ = file.WriteAt(block, offset)
					}
					ops.Add(1)
				}
			}
		}(time.Now().UnixNano() + int64(i))
	}
	go func() {
		wg.Wait()
	}()
}

func sleepOrCancel(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

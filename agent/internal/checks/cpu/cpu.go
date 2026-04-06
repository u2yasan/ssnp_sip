package cpu

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/u2yasan/ssnp_sip/agent/internal/policy"
)

type Result struct {
	Passed          bool
	NormalizedScore float64
}

func Run(ctx context.Context, profile policy.CPUProfile) Result {
	workers := min(profile.WorkerCap, runtime.NumCPU())
	if workers <= 0 {
		return Result{}
	}

	warmup := time.Duration(profile.WarmupSeconds) * time.Second
	measured := time.Duration(profile.MeasuredSeconds) * time.Second
	cooldown := time.Duration(profile.CooldownSeconds) * time.Second

	var count atomic.Uint64
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			runWorker(workerID, profile, &count, stop)
		}(i)
	}

	sleepOrCancel(ctx, warmup)
	count.Store(0)
	sleepOrCancel(ctx, measured)
	total := count.Load()
	sleepOrCancel(ctx, cooldown)
	close(stop)
	wg.Wait()

	score := float64(total) / math.Max(float64(profile.MeasuredSeconds), 1)
	return Result{
		Passed:          score >= profile.AcceptanceFloor.Minimum,
		NormalizedScore: score,
	}
}

func runWorker(workerID int, profile policy.CPUProfile, count *atomic.Uint64, stop <-chan struct{}) {
	var seq uint64 = uint64(workerID + 1)
	for {
		select {
		case <-stop:
			return
		default:
			seq = seq*6364136223846793005 + 1
			choice := float64(seq%1000) / 1000.0
			switch {
			case choice < profile.WorkloadMix.Hashing:
				doHash(seq)
			case choice < profile.WorkloadMix.Hashing+profile.WorkloadMix.Integer:
				doInteger(seq)
			default:
				doMatrix(seq)
			}
			count.Add(1)
		}
	}
}

func doHash(v uint64) {
	var buf [64]byte
	binary.LittleEndian.PutUint64(buf[:8], v)
	_ = sha256.Sum256(buf[:])
}

func doInteger(v uint64) {
	x := v
	for i := 0; i < 64; i++ {
		x = ((x << 7) ^ (x >> 3)) + uint64(i*i+1)
	}
	_ = x
}

func doMatrix(v uint64) {
	a := [9]float64{
		float64(v%7 + 1), 2, 3,
		4, 5, 6,
		7, 8, 9,
	}
	b := [9]float64{
		9, 8, 7,
		6, 5, 4,
		3, 2, 1,
	}
	var c [9]float64
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			for k := 0; k < 3; k++ {
				c[row*3+col] += a[row*3+k] * b[k*3+col]
			}
		}
	}
	_ = c
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

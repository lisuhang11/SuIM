// Package bbr implements a process-local adaptive limiter following the
// signals used by OpenIM's BBR limiter: CPU usage, in-flight requests,
// windowed pass count and windowed minimum response time.
package bbr

import (
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lisuhang11/SuIM/pkg/ratelimit/types"
)

var (
	gCPU  int64
	decay = 0.95

	lastTotal uint64
	lastIdle  uint64
)

func init() {
	go cpuProc()
}

// cpuProc samples /proc/stat every 500ms and keeps an EMA on 0-1000 scale.
func cpuProc() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer func() {
		if rec := recover(); rec != nil {
			go cpuProc()
		}
	}()
	defer ticker.Stop()

	for range ticker.C {
		usage := sampleCPU()
		if usage < 0 {
			continue
		}
		prev := atomic.LoadInt64(&gCPU)
		next := int64(float64(prev)*decay + float64(usage)*(1-decay))
		atomic.StoreInt64(&gCPU, next)
	}
}

func sampleCPU() int64 {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return -1
	}
	line := string(b)
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	fields := strings.Fields(line)
	if len(fields) < 6 || fields[0] != "cpu" {
		return -1
	}
	values := make([]uint64, 0, len(fields)-1)
	for _, field := range fields[1:] {
		v, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return -1
		}
		values = append(values, v)
	}
	total := uint64(0)
	for _, v := range values {
		total += v
	}
	idle := values[3] + values[4] // idle + iowait
	if total <= lastTotal || idle < lastIdle {
		lastTotal, lastIdle = total, idle
		return -1
	}
	deltaTotal := total - lastTotal
	deltaIdle := idle - lastIdle
	lastTotal, lastIdle = total, idle
	if deltaIdle > deltaTotal || deltaTotal == 0 {
		return -1
	}
	usage := int64(float64(deltaTotal-deltaIdle) / float64(deltaTotal) * 1000)
	if usage < 0 {
		return 0
	}
	if usage > 1000 {
		return 1000
	}
	return usage
}

type cpuGetter func() int64

type options struct {
	Window       time.Duration
	Bucket       int
	CPUThreshold int64
	CPUQuota     float64
	CPUGetter    cpuGetter
}

// Option customises a BBR limiter.
type Option func(*options)

// WithWindow sets the length of the sliding window.
func WithWindow(d time.Duration) Option {
	return func(o *options) { o.Window = d }
}

// WithBucket sets how many buckets the sliding window is split into.
func WithBucket(bucket int) Option {
	return func(o *options) { o.Bucket = bucket }
}

// WithCPUThreshold sets the CPU threshold on a 0-1000 scale.
func WithCPUThreshold(threshold int64) Option {
	return func(o *options) { o.CPUThreshold = threshold }
}

// WithCPUQuota scales the observed CPU value, mainly for CPU-limited pods.
func WithCPUQuota(quota float64) Option {
	return func(o *options) { o.CPUQuota = quota }
}

// WithCPUGetter replaces the CPU source. It is useful in tests.
func WithCPUGetter(getter func() int64) Option {
	return func(o *options) { o.CPUGetter = getter }
}

type bucket struct {
	pass    int64
	rtSum   int64
	rtCount int64
}

type rollingWindow struct {
	mu       sync.Mutex
	buckets  []bucket
	size     int
	duration time.Duration
	head     int
	headTime time.Time
}

func newRollingWindow(size int, duration time.Duration) *rollingWindow {
	if size < 1 {
		size = 1
	}
	if duration <= 0 {
		duration = time.Millisecond
	}
	return &rollingWindow{
		buckets:  make([]bucket, size),
		size:     size,
		duration: duration,
		headTime: time.Now().Truncate(duration),
	}
}

func (w *rollingWindow) advance(now time.Time) {
	steps := int(now.Sub(w.headTime) / w.duration)
	if steps <= 0 {
		return
	}
	if steps >= w.size {
		for i := range w.buckets {
			w.buckets[i] = bucket{}
		}
		w.head = 0
		w.headTime = now.Truncate(w.duration)
		return
	}
	for i := 0; i < steps; i++ {
		w.head = (w.head + 1) % w.size
		w.buckets[w.head] = bucket{}
	}
	w.headTime = w.headTime.Add(time.Duration(steps) * w.duration)
}

func (w *rollingWindow) add(pass int64, rt int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.advance(time.Now())
	if pass > 0 {
		w.buckets[w.head].pass += pass
	}
	if rt > 0 {
		w.buckets[w.head].rtSum += rt
		w.buckets[w.head].rtCount++
	}
}

func (w *rollingWindow) stats(now time.Time) (maxPass int64, minRT int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.advance(now)
	maxPass = 1
	for _, b := range w.buckets {
		if b.pass > maxPass {
			maxPass = b.pass
		}
		if b.rtCount <= 0 {
			continue
		}
		avg := b.rtSum / b.rtCount
		if avg > 0 && (minRT == 0 || avg < minRT) {
			minRT = avg
		}
	}
	if minRT <= 0 {
		minRT = 1
	}
	return maxPass, minRT
}

type counterCache struct {
	maxPass int64
	minRT   int64
	time    time.Time
}

// BBR is a process-local adaptive limiter.
type BBR struct {
	opts            options
	cpu             cpuGetter
	passStat        *rollingWindow
	rtStat          *rollingWindow
	inFlight        atomic.Int64
	prevDropTime    atomic.Int64
	cache           atomic.Value
	bucketDuration  time.Duration
	bucketPerSecond int64
}

var _ types.Limiter = (*BBR)(nil)

// NewBBRLimiter builds a limiter with OpenIM-compatible defaults.
func NewBBRLimiter(opts ...Option) *BBR {
	o := options{
		Window:       10 * time.Second,
		Bucket:       100,
		CPUThreshold: 800,
	}
	for _, apply := range opts {
		apply(&o)
	}
	if o.Window <= 0 {
		o.Window = 10 * time.Second
	}
	if o.Bucket <= 0 {
		o.Bucket = 100
	}
	bucketDuration := o.Window / time.Duration(o.Bucket)
	if bucketDuration <= 0 {
		bucketDuration = time.Millisecond
	}
	bucketPerSecond := int64(time.Second / bucketDuration)
	if bucketPerSecond < 1 {
		bucketPerSecond = 1
	}
	cpu := o.CPUGetter
	if cpu == nil {
		cpu = func() int64 { return atomic.LoadInt64(&gCPU) }
	}
	if o.CPUQuota != 0 {
		base := cpu
		cpu = func() int64 { return int64(float64(base()) * o.CPUQuota) }
	}
	return &BBR{
		opts:            o,
		cpu:             cpu,
		passStat:        newRollingWindow(o.Bucket, bucketDuration),
		rtStat:          newRollingWindow(o.Bucket, bucketDuration),
		bucketDuration:  bucketDuration,
		bucketPerSecond: bucketPerSecond,
	}
}

// Allow decides whether a request may proceed. Allowed requests must call the
// returned DoneFunc when they finish.
func (l *BBR) Allow() (types.DoneFunc, error) {
	if l.shouldDrop() {
		return nil, types.ErrLimitExceeded
	}
	l.inFlight.Add(1)
	start := time.Now()
	var once sync.Once
	return func(types.DoneInfo) {
		once.Do(func() {
			rt := int64((time.Since(start) + time.Millisecond - time.Nanosecond) / time.Millisecond)
			if rt > 0 {
				l.rtStat.add(0, rt)
			}
			l.inFlight.Add(-1)
			l.passStat.add(1, 0)
		})
	}, nil
}

func (l *BBR) shouldDrop() bool {
	now := time.Now().UnixNano()
	inFlight := l.inFlight.Load()
	if l.cpu() < l.opts.CPUThreshold {
		prev := l.prevDropTime.Load()
		if prev == 0 {
			return false
		}
		if now-prev <= int64(time.Second) {
			return inFlight > 1 && inFlight > l.maxPass()
		}
		l.prevDropTime.Store(0)
		return false
	}
	drop := inFlight > 1 && inFlight > l.maxInFlight()
	if drop && l.prevDropTime.Load() == 0 {
		l.prevDropTime.CompareAndSwap(0, now)
	}
	return drop
}

func (l *BBR) cachedStats() (int64, int64) {
	now := time.Now()
	if v := l.cache.Load(); v != nil {
		c := v.(counterCache)
		if now.Sub(c.time) < l.bucketDuration {
			return c.maxPass, c.minRT
		}
	}
	maxPass, _ := l.passStat.stats(now)
	_, minRT := l.rtStat.stats(now)
	if minRT <= 0 {
		minRT = 1
	}
	l.cache.Store(counterCache{maxPass: maxPass, minRT: minRT, time: now})
	return maxPass, minRT
}

func (l *BBR) maxPass() int64 {
	maxPass, _ := l.cachedStats()
	return maxPass
}

func (l *BBR) minRT() int64 {
	_, minRT := l.cachedStats()
	return minRT
}

func (l *BBR) maxInFlight() int64 {
	maxPass, minRT := l.cachedStats()
	value := math.Floor(float64(maxPass*minRT*l.bucketPerSecond)/1000.0) + 0.5
	if value < 0 {
		return 0
	}
	return int64(value)
}

// Stat returns the signals currently used by the limiter.
func (l *BBR) Stat() types.Stat {
	return types.Stat{
		CPU:         l.cpu(),
		InFlight:    l.inFlight.Load(),
		MaxInFlight: l.maxInFlight(),
		MinRT:       l.minRT(),
		MaxPass:     l.maxPass(),
	}
}

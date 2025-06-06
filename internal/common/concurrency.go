package common

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// ConcurrencyManager manages dynamic concurrency for operations
type ConcurrencyManager struct {
	mu                sync.RWMutex
	minWorkers        int
	maxWorkers        int
	currentWorkers    int32
	successRate       float64
	lastAdjustment    time.Time
	adjustmentPeriod  time.Duration
	metrics           *ConcurrencyMetrics
}

// ConcurrencyMetrics tracks performance metrics
type ConcurrencyMetrics struct {
	totalRequests    int64
	successfulOps    int64
	failedOps        int64
	rateLimitHits    int64
	avgResponseTime  int64 // in milliseconds
	lastResponseTime int64
}

// NewConcurrencyManager creates a new adaptive concurrency manager
func NewConcurrencyManager(minWorkers, maxWorkers int) *ConcurrencyManager {
	if minWorkers <= 0 {
		minWorkers = 1
	}
	if maxWorkers < minWorkers {
		maxWorkers = minWorkers * 10
	}

	return &ConcurrencyManager{
		minWorkers:       minWorkers,
		maxWorkers:       maxWorkers,
		currentWorkers:   int32(minWorkers),
		successRate:      1.0,
		lastAdjustment:   time.Now(),
		adjustmentPeriod: 10 * time.Second,
		metrics:          &ConcurrencyMetrics{},
	}
}

// GetOptimalConcurrency returns the current optimal concurrency level
func (cm *ConcurrencyManager) GetOptimalConcurrency() int {
	return int(atomic.LoadInt32(&cm.currentWorkers))
}

// RecordSuccess records a successful operation
func (cm *ConcurrencyManager) RecordSuccess(responseTime time.Duration) {
	atomic.AddInt64(&cm.metrics.totalRequests, 1)
	atomic.AddInt64(&cm.metrics.successfulOps, 1)
	atomic.StoreInt64(&cm.metrics.lastResponseTime, responseTime.Milliseconds())
	
	// Update average response time (simple moving average)
	currentAvg := atomic.LoadInt64(&cm.metrics.avgResponseTime)
	newAvg := (currentAvg*9 + responseTime.Milliseconds()) / 10
	atomic.StoreInt64(&cm.metrics.avgResponseTime, newAvg)
	
	cm.adjustConcurrency()
}

// RecordFailure records a failed operation
func (cm *ConcurrencyManager) RecordFailure(isRateLimit bool) {
	atomic.AddInt64(&cm.metrics.totalRequests, 1)
	atomic.AddInt64(&cm.metrics.failedOps, 1)
	
	if isRateLimit {
		atomic.AddInt64(&cm.metrics.rateLimitHits, 1)
		// Immediately reduce concurrency on rate limit
		cm.decreaseConcurrency(0.5) // Reduce by 50%
	}
	
	cm.adjustConcurrency()
}

// adjustConcurrency adjusts the concurrency based on metrics
func (cm *ConcurrencyManager) adjustConcurrency() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	// Don't adjust too frequently
	if time.Since(cm.lastAdjustment) < cm.adjustmentPeriod {
		return
	}
	
	total := atomic.LoadInt64(&cm.metrics.totalRequests)
	if total < 100 { // Need enough data
		return
	}
	
	successful := atomic.LoadInt64(&cm.metrics.successfulOps)
	// failed := atomic.LoadInt64(&cm.metrics.failedOps) // Currently unused
	rateLimits := atomic.LoadInt64(&cm.metrics.rateLimitHits)
	avgResponseTime := atomic.LoadInt64(&cm.metrics.avgResponseTime)
	
	// Calculate success rate
	successRate := float64(successful) / float64(total)
	
	// Decide whether to increase or decrease
	current := atomic.LoadInt32(&cm.currentWorkers)
	
	if rateLimits > 0 {
		// Recent rate limits, decrease significantly
		cm.decreaseConcurrency(0.7)
	} else if successRate > 0.95 && avgResponseTime < 500 {
		// High success rate and fast responses, increase
		cm.increaseConcurrency(1.2)
	} else if successRate < 0.9 || avgResponseTime > 2000 {
		// Lower success rate or slow responses, decrease
		cm.decreaseConcurrency(0.9)
	}
	
	// Reset metrics for next period
	atomic.StoreInt64(&cm.metrics.totalRequests, 0)
	atomic.StoreInt64(&cm.metrics.successfulOps, 0)
	atomic.StoreInt64(&cm.metrics.failedOps, 0)
	atomic.StoreInt64(&cm.metrics.rateLimitHits, 0)
	
	cm.successRate = successRate
	cm.lastAdjustment = time.Now()
	
	newWorkers := atomic.LoadInt32(&cm.currentWorkers)
	if newWorkers != current {
		// Log adjustment (could be replaced with proper logging)
		// fmt.Printf("Concurrency adjusted: %d -> %d (success rate: %.2f%%, avg response: %dms)\n", 
		//     current, newWorkers, successRate*100, avgResponseTime)
	}
}

// increaseConcurrency increases the worker count by a factor
func (cm *ConcurrencyManager) increaseConcurrency(factor float64) {
	current := atomic.LoadInt32(&cm.currentWorkers)
	newCount := int32(float64(current) * factor)
	
	if newCount > int32(cm.maxWorkers) {
		newCount = int32(cm.maxWorkers)
	}
	
	atomic.StoreInt32(&cm.currentWorkers, newCount)
}

// decreaseConcurrency decreases the worker count by a factor
func (cm *ConcurrencyManager) decreaseConcurrency(factor float64) {
	current := atomic.LoadInt32(&cm.currentWorkers)
	newCount := int32(float64(current) * factor)
	
	if newCount < int32(cm.minWorkers) {
		newCount = int32(cm.minWorkers)
	}
	
	atomic.StoreInt32(&cm.currentWorkers, newCount)
}

// GetMetrics returns a copy of current metrics
func (cm *ConcurrencyManager) GetMetrics() ConcurrencyMetrics {
	return ConcurrencyMetrics{
		totalRequests:    atomic.LoadInt64(&cm.metrics.totalRequests),
		successfulOps:    atomic.LoadInt64(&cm.metrics.successfulOps),
		failedOps:        atomic.LoadInt64(&cm.metrics.failedOps),
		rateLimitHits:    atomic.LoadInt64(&cm.metrics.rateLimitHits),
		avgResponseTime:  atomic.LoadInt64(&cm.metrics.avgResponseTime),
		lastResponseTime: atomic.LoadInt64(&cm.metrics.lastResponseTime),
	}
}

// AdaptiveWorkerPool manages a pool of workers with dynamic sizing
type AdaptiveWorkerPool struct {
	ctx              context.Context
	cancel           context.CancelFunc
	workChan         chan interface{}
	resultChan       chan interface{}
	errorChan        chan error
	concurrencyMgr   *ConcurrencyManager
	workerFunc       func(context.Context, interface{}) (interface{}, error)
	activeWorkers    int32
	wg               sync.WaitGroup
}

// NewAdaptiveWorkerPool creates a new adaptive worker pool
func NewAdaptiveWorkerPool(ctx context.Context, minWorkers, maxWorkers int, 
	workerFunc func(context.Context, interface{}) (interface{}, error)) *AdaptiveWorkerPool {
	
	poolCtx, cancel := context.WithCancel(ctx)
	
	pool := &AdaptiveWorkerPool{
		ctx:            poolCtx,
		cancel:         cancel,
		workChan:       make(chan interface{}, maxWorkers*2),
		resultChan:     make(chan interface{}, maxWorkers),
		errorChan:      make(chan error, maxWorkers),
		concurrencyMgr: NewConcurrencyManager(minWorkers, maxWorkers),
		workerFunc:     workerFunc,
	}
	
	// Start initial workers
	pool.adjustWorkers()
	
	// Start worker adjustment routine
	go pool.monitorAndAdjust()
	
	return pool
}

// Submit submits work to the pool
func (p *AdaptiveWorkerPool) Submit(work interface{}) error {
	select {
	case p.workChan <- work:
		return nil
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}

// Results returns the result channel
func (p *AdaptiveWorkerPool) Results() <-chan interface{} {
	return p.resultChan
}

// Errors returns the error channel
func (p *AdaptiveWorkerPool) Errors() <-chan error {
	return p.errorChan
}

// Close shuts down the worker pool
func (p *AdaptiveWorkerPool) Close() error {
	p.cancel()
	close(p.workChan)
	p.wg.Wait()
	close(p.resultChan)
	close(p.errorChan)
	return nil
}

// adjustWorkers adjusts the number of active workers
func (p *AdaptiveWorkerPool) adjustWorkers() {
	targetWorkers := p.concurrencyMgr.GetOptimalConcurrency()
	currentWorkers := atomic.LoadInt32(&p.activeWorkers)
	
	if targetWorkers > int(currentWorkers) {
		// Start more workers
		for i := currentWorkers; i < int32(targetWorkers); i++ {
			p.wg.Add(1)
			go p.worker()
			atomic.AddInt32(&p.activeWorkers, 1)
		}
	}
	// Note: We don't stop workers here, they'll naturally exit when there's no work
}

// worker is the main worker routine
func (p *AdaptiveWorkerPool) worker() {
	defer p.wg.Done()
	defer atomic.AddInt32(&p.activeWorkers, -1)
	
	for {
		select {
		case work, ok := <-p.workChan:
			if !ok {
				return
			}
			
			start := time.Now()
			result, err := p.workerFunc(p.ctx, work)
			duration := time.Since(start)
			
			if err != nil {
				p.concurrencyMgr.RecordFailure(isRateLimitError(err))
				select {
				case p.errorChan <- err:
				case <-p.ctx.Done():
					return
				}
			} else {
				p.concurrencyMgr.RecordSuccess(duration)
				select {
				case p.resultChan <- result:
				case <-p.ctx.Done():
					return
				}
			}
			
		case <-p.ctx.Done():
			return
		}
	}
}

// monitorAndAdjust periodically adjusts the worker pool size
func (p *AdaptiveWorkerPool) monitorAndAdjust() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			p.adjustWorkers()
		case <-p.ctx.Done():
			return
		}
	}
}

// isRateLimitError checks if an error is a rate limit error
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	// Check for common rate limit indicators
	errStr := err.Error()
	return contains(errStr, "429") || contains(errStr, "rate limit") || 
		contains(errStr, "too many requests")
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		(s == substr || 
		 len(s) > len(substr) && 
		 (stringContains(toLowerCase(s), toLowerCase(substr))))
}

// Simple string utilities to avoid importing strings package
func toLowerCase(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
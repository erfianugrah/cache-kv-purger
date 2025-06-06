package common

import (
	"bytes"
	"strings"
	"sync"
)

// MemoryPools provides centralized memory pooling for the application
var MemoryPools = &memoryPools{
	stringBuilders: sync.Pool{
		New: func() interface{} {
			return &strings.Builder{}
		},
	},
	byteBuffers: sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	},
	smallSlices: sync.Pool{
		New: func() interface{} {
			s := make([]string, 0, 100)
			return &s
		},
	},
	largeSlices: sync.Pool{
		New: func() interface{} {
			s := make([]string, 0, 1000)
			return &s
		},
	},
	keyValuePairs: sync.Pool{
		New: func() interface{} {
			s := make([]interface{}, 0, 1000)
			return &s
		},
	},
}

type memoryPools struct {
	stringBuilders sync.Pool
	byteBuffers    sync.Pool
	smallSlices    sync.Pool
	largeSlices    sync.Pool
	keyValuePairs  sync.Pool
}

// GetStringBuilder gets a string builder from the pool
func (p *memoryPools) GetStringBuilder() *strings.Builder {
	sb := p.stringBuilders.Get().(*strings.Builder)
	sb.Reset()
	return sb
}

// PutStringBuilder returns a string builder to the pool
func (p *memoryPools) PutStringBuilder(sb *strings.Builder) {
	if sb.Cap() > 1024*1024 { // Don't pool very large builders (>1MB)
		return
	}
	p.stringBuilders.Put(sb)
}

// GetByteBuffer gets a byte buffer from the pool
func (p *memoryPools) GetByteBuffer() *bytes.Buffer {
	buf := p.byteBuffers.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// PutByteBuffer returns a byte buffer to the pool
func (p *memoryPools) PutByteBuffer(buf *bytes.Buffer) {
	if buf.Cap() > 1024*1024 { // Don't pool very large buffers (>1MB)
		return
	}
	p.byteBuffers.Put(buf)
}

// GetSmallSlice gets a small string slice from the pool (capacity: 100)
func (p *memoryPools) GetSmallSlice() *[]string {
	s := p.smallSlices.Get().(*[]string)
	*s = (*s)[:0] // Reset length but keep capacity
	return s
}

// PutSmallSlice returns a small string slice to the pool
func (p *memoryPools) PutSmallSlice(s *[]string) {
	if cap(*s) > 500 { // Don't pool if it grew too large
		return
	}
	p.smallSlices.Put(s)
}

// GetLargeSlice gets a large string slice from the pool (capacity: 1000)
func (p *memoryPools) GetLargeSlice() *[]string {
	s := p.largeSlices.Get().(*[]string)
	*s = (*s)[:0] // Reset length but keep capacity
	return s
}

// PutLargeSlice returns a large string slice to the pool
func (p *memoryPools) PutLargeSlice(s *[]string) {
	if cap(*s) > 10000 { // Don't pool if it grew too large
		return
	}
	p.largeSlices.Put(s)
}

// GetKeyValueSlice gets a slice for key-value pairs from the pool
func (p *memoryPools) GetKeyValueSlice() *[]interface{} {
	s := p.keyValuePairs.Get().(*[]interface{})
	*s = (*s)[:0] // Reset length but keep capacity
	return s
}

// PutKeyValueSlice returns a key-value slice to the pool
func (p *memoryPools) PutKeyValueSlice(s *[]interface{}) {
	if cap(*s) > 10000 { // Don't pool if it grew too large
		return
	}
	// Clear references to help GC
	for i := range *s {
		(*s)[i] = nil
	}
	p.keyValuePairs.Put(s)
}

// PooledBatchProcessor represents a memory-efficient batch processor using pools
type PooledBatchProcessor struct {
	batchSize int
	pool      *sync.Pool
}

// NewPooledBatchProcessor creates a new batch processor with pooled batches
func NewPooledBatchProcessor(batchSize int) *PooledBatchProcessor {
	return &PooledBatchProcessor{
		batchSize: batchSize,
		pool: &sync.Pool{
			New: func() interface{} {
				return make([]interface{}, 0, batchSize)
			},
		},
	}
}

// GetBatch gets a batch from the pool
func (bp *PooledBatchProcessor) GetBatch() []interface{} {
	batch := bp.pool.Get().([]interface{})
	return batch[:0] // Reset length but keep capacity
}

// PutBatch returns a batch to the pool
func (bp *PooledBatchProcessor) PutBatch(batch []interface{}) {
	if cap(batch) > bp.batchSize*2 { // Don't pool if it grew too large
		return
	}
	// Clear references to help GC
	for i := range batch {
		batch[i] = nil
	}
	bp.pool.Put(batch)
}

// StringPool provides efficient string interning for frequently used strings
type StringPool struct {
	mu    sync.RWMutex
	pool  map[string]string
	maxSize int
}

// NewStringPool creates a new string pool with a maximum size
func NewStringPool(maxSize int) *StringPool {
	return &StringPool{
		pool:    make(map[string]string),
		maxSize: maxSize,
	}
}

// Intern returns an interned version of the string
func (sp *StringPool) Intern(s string) string {
	sp.mu.RLock()
	if interned, ok := sp.pool[s]; ok {
		sp.mu.RUnlock()
		return interned
	}
	sp.mu.RUnlock()

	sp.mu.Lock()
	defer sp.mu.Unlock()

	// Double-check after acquiring write lock
	if interned, ok := sp.pool[s]; ok {
		return interned
	}

	// Add to pool if not at capacity
	if len(sp.pool) < sp.maxSize {
		sp.pool[s] = s
		return s
	}

	// Pool is full, just return the original string
	return s
}

// Clear empties the string pool
func (sp *StringPool) Clear() {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.pool = make(map[string]string)
}
package kv

import (
	"container/list"
	"fmt"
	"sync"
	"time"
	
	"cache-kv-purger/internal/api"
)

// CacheEntry represents a cached item with expiration
type CacheEntry struct {
	Key       string
	Value     interface{}
	ExpiresAt time.Time
	Size      int64
}

// LRUCache implements a thread-safe LRU cache with TTL support
type LRUCache struct {
	maxSize    int64
	maxEntries int
	ttl        time.Duration
	size       int64
	
	mu      sync.RWMutex
	entries map[string]*list.Element
	lru     *list.List
	
	// Metrics
	hits   int64
	misses int64
}

// NewLRUCache creates a new LRU cache
func NewLRUCache(maxEntries int, maxSize int64, ttl time.Duration) *LRUCache {
	if maxEntries <= 0 {
		maxEntries = 1000
	}
	if maxSize <= 0 {
		maxSize = 100 * 1024 * 1024 // 100MB default
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	
	return &LRUCache{
		maxEntries: maxEntries,
		maxSize:    maxSize,
		ttl:        ttl,
		entries:    make(map[string]*list.Element),
		lru:        list.New(),
	}
}

// Get retrieves a value from the cache
func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	elem, exists := c.entries[key]
	c.mu.RUnlock()
	
	if !exists {
		c.incrementMisses()
		return nil, false
	}
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	entry := elem.Value.(*CacheEntry)
	
	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		c.removeElement(elem)
		c.misses++
		return nil, false
	}
	
	// Move to front (most recently used)
	c.lru.MoveToFront(elem)
	c.hits++
	
	return entry.Value, true
}

// Set adds or updates a value in the cache
func (c *LRUCache) Set(key string, value interface{}, size int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Check if already exists
	if elem, exists := c.entries[key]; exists {
		// Update existing entry
		entry := elem.Value.(*CacheEntry)
		c.size -= entry.Size
		entry.Value = value
		entry.Size = size
		entry.ExpiresAt = time.Now().Add(c.ttl)
		c.size += size
		c.lru.MoveToFront(elem)
		return
	}
	
	// Create new entry
	entry := &CacheEntry{
		Key:       key,
		Value:     value,
		Size:      size,
		ExpiresAt: time.Now().Add(c.ttl),
	}
	
	elem := c.lru.PushFront(entry)
	c.entries[key] = elem
	c.size += size
	
	// Evict if necessary
	c.evict()
}

// Delete removes a key from the cache
func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if elem, exists := c.entries[key]; exists {
		c.removeElement(elem)
	}
}

// Clear removes all entries from the cache
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.entries = make(map[string]*list.Element)
	c.lru = list.New()
	c.size = 0
}

// Stats returns cache statistics
func (c *LRUCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return CacheStats{
		Entries:  len(c.entries),
		Size:     c.size,
		MaxSize:  c.maxSize,
		Hits:     c.hits,
		Misses:   c.misses,
		HitRatio: c.calculateHitRatio(),
	}
}

// CacheStats contains cache statistics
type CacheStats struct {
	Entries  int
	Size     int64
	MaxSize  int64
	Hits     int64
	Misses   int64
	HitRatio float64
}

// removeElement removes an element from the cache (must be called with lock held)
func (c *LRUCache) removeElement(elem *list.Element) {
	entry := elem.Value.(*CacheEntry)
	delete(c.entries, entry.Key)
	c.lru.Remove(elem)
	c.size -= entry.Size
}

// evict removes least recently used entries until within limits
func (c *LRUCache) evict() {
	for c.size > c.maxSize || len(c.entries) > c.maxEntries {
		elem := c.lru.Back()
		if elem == nil {
			break
		}
		c.removeElement(elem)
	}
}

// incrementMisses safely increments miss counter
func (c *LRUCache) incrementMisses() {
	c.mu.Lock()
	c.misses++
	c.mu.Unlock()
}

// calculateHitRatio calculates the cache hit ratio
func (c *LRUCache) calculateHitRatio() float64 {
	total := c.hits + c.misses
	if total == 0 {
		return 0
	}
	return float64(c.hits) / float64(total)
}

// MetadataCache provides caching specifically for KV metadata
type MetadataCache struct {
	cache *LRUCache
}

// NewMetadataCache creates a new metadata cache
func NewMetadataCache(maxEntries int, maxSizeMB int, ttl time.Duration) *MetadataCache {
	return &MetadataCache{
		cache: NewLRUCache(maxEntries, int64(maxSizeMB)*1024*1024, ttl),
	}
}

// GetMetadata retrieves metadata from cache
func (mc *MetadataCache) GetMetadata(key string) (*KeyValueMetadata, bool) {
	value, found := mc.cache.Get(key)
	if !found {
		return nil, false
	}
	
	metadata, ok := value.(*KeyValueMetadata)
	if !ok {
		return nil, false
	}
	
	return metadata, true
}

// SetMetadata stores metadata in cache
func (mc *MetadataCache) SetMetadata(key string, metadata *KeyValueMetadata) {
	if metadata == nil {
		return
	}
	
	// Estimate size (rough approximation)
	size := int64(len(key) + 100) // Base overhead
	for k, v := range *metadata {
		size += int64(len(k))
		if str, ok := v.(string); ok {
			size += int64(len(str))
		} else {
			size += 50 // Estimate for other types
		}
	}
	
	mc.cache.Set(key, metadata, size)
}

// Clear clears the metadata cache
func (mc *MetadataCache) Clear() {
	mc.cache.Clear()
}

// Stats returns cache statistics
func (mc *MetadataCache) Stats() CacheStats {
	return mc.cache.Stats()
}

// GlobalMetadataCache is a singleton metadata cache instance
var (
	globalMetadataCache *MetadataCache
	cacheOnce           sync.Once
)

// GetGlobalMetadataCache returns the global metadata cache instance
func GetGlobalMetadataCache() *MetadataCache {
	cacheOnce.Do(func() {
		// Create with sensible defaults
		globalMetadataCache = NewMetadataCache(
			10000,           // 10k entries max
			50,              // 50MB max size
			5*time.Minute,   // 5 minute TTL
		)
	})
	return globalMetadataCache
}

// CachedGetKeyMetadata gets metadata with caching
func CachedGetKeyMetadata(client *api.Client, accountID, namespaceID, key string) (*KeyValueMetadata, error) {
	cache := GetGlobalMetadataCache()
	
	// Create cache key
	cacheKey := fmt.Sprintf("%s:%s:%s", accountID, namespaceID, key)
	
	// Check cache first
	if metadata, found := cache.GetMetadata(cacheKey); found {
		return metadata, nil
	}
	
	// Cache miss - fetch from API
	kvp, err := GetKeyWithMetadata(client, accountID, namespaceID, key)
	if err != nil {
		return nil, err
	}
	
	// Store in cache
	cache.SetMetadata(cacheKey, kvp.Metadata)
	
	return kvp.Metadata, nil
}

// CachedBulkGetMetadata fetches metadata for multiple keys with caching
func CachedBulkGetMetadata(client *api.Client, accountID, namespaceID string, keys []string) (map[string]*KeyValueMetadata, error) {
	if len(keys) == 0 {
		return make(map[string]*KeyValueMetadata), nil
	}
	
	cache := GetGlobalMetadataCache()
	results := make(map[string]*KeyValueMetadata)
	var uncachedKeys []string
	
	// Check cache for each key
	for _, key := range keys {
		cacheKey := fmt.Sprintf("%s:%s:%s", accountID, namespaceID, key)
		if metadata, found := cache.GetMetadata(cacheKey); found {
			results[key] = metadata
		} else {
			uncachedKeys = append(uncachedKeys, key)
		}
	}
	
	// If all keys were cached, return immediately
	if len(uncachedKeys) == 0 {
		return results, nil
	}
	
	// Fetch uncached keys
	uncachedResults, err := BulkGetMetadata(client, accountID, namespaceID, uncachedKeys)
	if err != nil {
		// Return partial results on error
		return results, err
	}
	
	// Merge results and update cache
	for key, metadata := range uncachedResults {
		results[key] = metadata
		cacheKey := fmt.Sprintf("%s:%s:%s", accountID, namespaceID, key)
		cache.SetMetadata(cacheKey, metadata)
	}
	
	return results, nil
}
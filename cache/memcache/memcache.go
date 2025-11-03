package memcache

import (
	"reflect"
	"sync"
	"time"
)

/*
	Memory cache for arbirtary data.
	Size-based eviction (no TTL).
	On eviction deletes LRU items amongst those older than `keepFirst` minutes.

	TODO: add TTL and batch eviction.
*/

const keepFirst = time.Minute * 5 // no-eviction time

type cacheRec struct {
	Value       interface{} // 16
	ContentType string      // 16
	AddedAt     int64       // 8
	UseCount    int         // 8
}

type cache struct {
	maxSize int
	size    int
	storage map[string]*cacheRec
	sync.RWMutex
}

type Cache interface {
	Get(key string) (interface{}, string, bool) // value, content-type, found
	Set(key string, value interface{}, contentType ...string)
	Del(key string)
	Cleanup()
	Size() int
}

func RecordSize() int {
	return valueSize(cacheRec{})
}

// New creates new memory cache with given maximum size in bytes
func New(size int) Cache {
	return &cache{
		maxSize: size,
		size:    0,
		storage: make(map[string]*cacheRec),
	}
}

// Get retrieves value from cache, nil if not found.
func (c *cache) Get(key string) (interface{}, string, bool) {
	c.Lock()
	defer c.Unlock()
	if v, found := c.storage[key]; found {
		c.storage[key].UseCount++
		return v.Value, v.ContentType, true
	}
	return nil, "", false
}

// Set stores a value in cache, evicting stale elements if needed. Can skip storing if the cache is full and no evictable entries found.
func (c *cache) Set(key string, value interface{}, contentType ...string) {
	c.Lock()
	defer c.Unlock()

	ct := ""
	for _, v := range contentType {
		ct = v
		break
	}
	prec, found := c.storage[key]
	useCount := 0
	addedAt := time.Now().Unix()
	if found {
		useCount = prec.UseCount
		addedAt = prec.AddedAt
		c.size -= valueSize(*prec)
	}

	rec := cacheRec{Value: value, ContentType: ct, UseCount: useCount + 1, AddedAt: addedAt}
	sz := valueSize(rec)

	for c.size+sz > c.maxSize {
		if !c.evict() {
			return
		}
	}
	c.storage[key] = &rec
	c.size += sz
}

// Del deletes cache entry if exists.
func (c *cache) Del(key string) {
	c.Lock()
	defer c.Unlock()
	if v, found := c.storage[key]; found {
		c.size -= valueSize(v)
		delete(c.storage, key)
	}
}

// Cleanup empties cache.
func (c *cache) Cleanup() {
	c.Lock()
	defer c.Unlock()
	for k := range c.storage {
		delete(c.storage, k)
	}
}

// Size returns current cache size.
func (c *cache) Size() int {
	c.RLock()
	defer c.RUnlock()
	return c.size
}

// evict removes one less used stale entry.
// It has O(n) complexity, but we don't care for it now.
func (c *cache) evict() bool {
	var prey *cacheRec
	var key string
	from := time.Now().Add(-keepFirst).Unix()
	for k, v := range c.storage {
		if v.AddedAt < from && (prey == nil || v.UseCount < prey.UseCount) {
			prey = v
			key = k
		}
	}
	if prey != nil {
		sz := valueSize(*prey)
		c.size -= sz
		delete(c.storage, key)
		return true
	}
	return false
}

// stale moves cache entry back in time so it can be evicted (test only function)
func (c *cache) stale(key string) {
	c.Lock()
	defer c.Unlock()
	if v, found := c.storage[key]; found {
		c.storage[key] = &cacheRec{Value: v.Value, UseCount: v.UseCount, AddedAt: time.Now().Add(-keepFirst * 2).Unix()}
	}
}

func valueSize(value interface{}) int {
	return int(reflect.TypeOf(value).Size())
}

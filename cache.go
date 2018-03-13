package cafs

import (
	"time"

	"github.com/ipfs/go-datastore"
)

// Cache is the interface for wrapping a cafs, cache behaviour will vary from case-to-case
type Cache interface {
	// Caches must fully implement filestores, this should always delegate operations
	// to the underlying store at some point in each Filestore method call
	Filestore
	// Filestore must return the underlying filestore
	Filestore() Filestore
	// Cache explicitly adds a key to the Cache. it should be retrievable
	// by the underlying store
	Cache(key datastore.Key) error
	// Uncache explicitly removes a key from the cache
	Uncache(key datastore.Key) error
}

// TTECache is a cache that allows storing with an expiry
type TTECache interface {
	// TTECache implements the cache interface
	Cache
	// CacheTTE caches a key with a time-to-expiry
	CacheTTE(key datastore.Key, tte time.Duration) error
}

// NewCacheFunc is a generic function for creating a cache
// More sophisticated functions that configure caches & take lots of implementation-specific
// details can return a NewCacheFunc instead of the cache itself to conform to this signature
type NewCacheFunc func(fs Filestore) Cache

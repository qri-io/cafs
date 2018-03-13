package gcloud

import (
	"context"
	"fmt"
	"io"

	"cloud.google.com/go/storage"
	"github.com/ipfs/go-datastore"
	"github.com/qri-io/cafs"
)

// CacheCfg configures cache behaviour
type CacheCfg struct {
	// Setting Full == true will cache everything that is Put() into the store,
	// and check the cache on all Get()/Has() requests (falling back to the store)
	Full bool
	// Async will skip waiting for writes to cache
	AsyncWrite bool
	// Set the kind of key to read & write from the store. cannot be empty
	BucketName string

	// MaxFileSize specifies how large a byte slice can be before it
	// TODO - not yet implemented
	// MaxFileSize int64
}

// Cache implements the cafs.Cache interface using a google cloud datastore
type Cache struct {
	ctx    context.Context
	fs     cafs.Filestore
	client *storage.Client
	cfg    *CacheCfg
}

// NewCache creates a function that can create new cache from a filestore
func NewCacheFunc(ctx context.Context, cli *storage.Client, opts ...func(*CacheCfg)) cafs.NewCacheFunc {
	cfg := &CacheCfg{}
	for _, opt := range opts {
		opt(cfg)
	}

	return cafs.NewCacheFunc(func(fs cafs.Filestore) cafs.Cache {
		return Cache{
			ctx:    ctx,
			fs:     fs,
			client: cli,
			cfg:    cfg,
		}
	})
}

// Put places a file in the store
func (c Cache) Put(file cafs.File, pin bool) (key datastore.Key, err error) {
	key, err = c.fs.Put(file, pin)
	if err != nil {
		return
	}

	if c.shouldCache(file) {
		if c.cfg.AsyncWrite {
			go func() {
				err := c.putCache(key)
				if err != nil {
					fmt.Errorf("error placing in cache: %s", err.Error())
				}
			}()
		}
		err = c.putCache(key)
	}

	return key, err
}

// Get
func (c Cache) Get(key datastore.Key) (file cafs.File, err error) {
	return c.fs.Get(key)
}

// Has checks for the presence of a key
func (c Cache) Has(key datastore.Key) (bool, error) {
	return c.fs.Has(key)
}

// Delete removes a key from the store, possibly affecting the cache
func (c Cache) Delete(key datastore.Key) error {
	return c.fs.Delete(key)
}

// PathPrefix returns the prefix of the underlying store
func (c Cache) PathPrefix() string {
	return c.fs.PathPrefix()
}

// NewAdder proxies the store NewAdder method
func (c Cache) NewAdder(pin, wrap bool) (cafs.Adder, error) {
	return c.fs.NewAdder(pin, wrap)
	// fsAdder, err := c.fs.NewAdder(pin, wrap)
	// if err != nil {
	// 	return nil, err
	// }

	// adder := &adder{
	// 	cache: &c,
	// 	adder: fsAdder,
	// 	out:   make(chan cafs.AddedFile, 9),
	// }

	// go func() {
	// 	for af := range fsAdder.Added() {
	// 		adder.out <- af
	// 	}
	// }()

	// return adder
}

// Adder wraps a coreunix adder to conform to the cafs adder interface
// type adder struct {
// 	cache *Cache
// 	adder cafs.Adder
// 	out   chan cafs.AddedFile
// }

// func (a *adder) AddFile(f cafs.File) error {
// 	if a.cache.shouldCache(f) {
// 		pr, pw := io.Pipe()
// 		r := io.TeeReader(f, pw)
// 	}

// 	// a.adder.AddFile(f cafs.File)
// 	// // path, err := a.z.Put(f, a.pin)
// 	// if err != nil {
// 	// 	fmt.Errorf("error putting file in mapstore: %s", err.Error())
// 	// 	return err
// 	// }
// 	// a.out <- cafs.AddedFile{
// 	// 	Path:  path,
// 	// 	Name:  f.FileName(),
// 	// 	Bytes: 0,
// 	// 	Hash:  path.String(),
// 	// }
// 	return nil
// }

// func (a *adder) Added() chan cafs.AddedFile {
// 	return a.out
// }

// func (a *adder) Close() error {
// 	close(a.out)
// 	return nil
// }

// Filestore returns
func (c Cache) Filestore() cafs.Filestore {
	return c.fs
}

// Cache
func (c Cache) Cache(key datastore.Key) error {
	return c.putCache(key)
}

// Uncache removes a cache from the store
func (c Cache) Uncache(key datastore.Key) error {
	return c.deleteCache(key)
}

func (c Cache) shouldCache(file cafs.File) bool {
	return c.cfg.Full
}

//
func (c Cache) putCache(key datastore.Key) (err error) {
	file, err := c.fs.Get(key)
	if err != nil {
		return err
	}

	w := c.object(key).NewWriter(c.ctx)
	_, err = io.Copy(w, file)
	return
}

//
func (c Cache) getCache(key datastore.Key) (file cafs.File, err error) {
	obj := c.object(key)
	r, err := obj.NewReader(c.ctx)
	if err != nil {
		return nil, err
	}

	return cafs.NewMemfileReader(key.BaseNamespace(), r), err
}

func (c Cache) deleteCache(key datastore.Key) error {
	return c.object(key).Delete(c.ctx)
}

func (c Cache) object(key datastore.Key) *storage.ObjectHandle {
	return c.bucket().Object(key.String())
}

//
func (c Cache) bucket() *storage.BucketHandle {
	return c.client.Bucket(c.cfg.BucketName)
}

package test

import (
	"bytes"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/qri-io/cafs"
	"github.com/qri-io/cafs/memfs"
)

func RunCacheTests(cf cafs.NewCacheFunc, t *testing.T) {
	fs := memfs.NewMapstore()
	cache := cf(fs)

	Filestore(cf, t)

	for _, test := range []func(cafs.Cache, *testing.T){
		File,
		Directory,
		Adder,
	} {
		test(cache, t)
	}
}

func Filestore(cf cafs.NewCacheFunc, t *testing.T) {
	key := datastore.NewKey("/foo")
	mf := memfs.NewMemfileBytes("foo", []byte("foo"))
	mem := memfs.NewMapstore()
	key, err := mem.Put(mf, false)
	if err != nil {
		t.Errorf("error configuring test MapStore: %s", err.Error())
		return
	}

	cache := cf(mem)

	if _, err := cache.Get(key); err != nil {
		t.Errorf("cache.Filestore didn't return passed-in filestore. %v != %v", mem, cache)
	}
}

func File(cache cafs.Cache, t *testing.T) {
	fs := cache.Filestore()
	fdata := []byte("foo")
	file := memfs.NewMemfileBytes("file.txt", fdata)

	key, err := fs.Put(file, false)
	if err != nil {
		t.Errorf("Filestore.Put(%s) error: %s", file.FileName(), err.Error())
		return
	}

	file = memfs.NewMemfileBytes("file.txt", fdata)
	cacheKey, err := cache.Put(file, false)
	if err != nil {
		t.Errorf("Cache.Put(%s) error: %s", file.FileName(), err.Error())
		return
	}

	if !key.Equal(cacheKey) {
		t.Errorf("Filestore.Put & Cache.Put returned different keys. fs: %s, cache: %s", key.String(), cacheKey.String())
		return
	}

	out, err := cache.Get(key)
	if err != nil {
		t.Errorf("Cache.Get a newly-put file error: %s", err.Error())
		return
	}

	p := make([]byte, len(fdata))
	if _, err := out.Read(p); err != nil {
		t.Errorf("error reading file: %s", err.Error())
		return
	}

	if !bytes.Equal(fdata, p) {
		t.Errorf("read byte mismatch: %s", err.Error())
		return
	}
}

func Directory(cache cafs.Cache, t *testing.T) {
	// TODO :/
}

func Adder(cache cafs.Cache, t *testing.T) {
	// TODO :/
}

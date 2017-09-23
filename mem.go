package cafs

import (
	"crypto/sha256"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-ipfs/commands/files"
	"github.com/jbenet/go-base58"
	"github.com/multiformats/go-multihash"
	"io"
	"math/rand"
)

// NewMamstore allocates an instance of a mapstore
func NewMapstore() Filestore {
	return MapStore{}
}

// MapStore implements Filestore in-memory as a map. This thing needs attention.
// TODO - fixme
type MapStore map[datastore.Key]files.File

func (m MapStore) Put(file files.File, pin bool) (key datastore.Key, err error) {
	// shhhhhh, this is a cheap hack for content-addressing
	// hash, err := hashBytes(data)

	hash, err := hashBytes(randBytes(40))
	if err != nil {
		return
	}

	key = datastore.NewKey("/map/" + hash)
	m[key] = file
	return
}

func (m MapStore) Get(key datastore.Key) (files.File, error) {
	if m[key] == nil {
		return nil, datastore.ErrNotFound
	}
	return m[key], nil
}

func (m MapStore) Has(key datastore.Key) (exists bool, err error) {
	if m[key] == nil {
		return false, nil
	}
	return true, nil
}

func (m MapStore) Delete(key datastore.Key) error {
	m[key] = nil
	return nil
}

func (m MapStore) NewAdder(pin, wrap bool) (Adder, error) {
	addedOut := make(chan AddedFile, 8)
	return &adder{
		mapstore: m,
		out:      addedOut,
	}, nil
}

// Adder wraps a coreunix adder to conform to the cafs adder interface
type adder struct {
	mapstore MapStore
	out      chan AddedFile
}

func (a *adder) AddFile(f files.File) error {
	if f.IsDirectory() {
		for {
			file, err := f.NextFile()
			if err == io.EOF {
				return nil
			}
			a.AddFile(file)
		}
	} else {
		// data, err := ioutil.ReadAll(f)
		// if err != nil {
		// 	return err
		// }
		// hash, err := hashBytes(data)
		// if err != nil {
		// 	return err
		// }

		hash, err := hashBytes(randBytes(40))
		if err != nil {
			return err
		}

		path := datastore.NewKey("/map/" + hash)
		a.mapstore[path] = f
		a.out <- AddedFile{
			Path:  path,
			Name:  f.FileName(),
			Bytes: 0,
			Hash:  hash,
		}
	}

	return nil
}

func (a *adder) Added() chan AddedFile {
	return a.out
}
func (a *adder) Close() error {
	close(a.out)
	return nil
}

func hashBytes(data []byte) (hash string, err error) {
	h := sha256.New()
	if _, err = h.Write(data); err != nil {
		return
	}
	mhBuf, err := multihash.Encode(h.Sum(nil), multihash.SHA2_256)
	if err != nil {
		return
	}
	hash = base58.Encode(mhBuf)
	return
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return b
}

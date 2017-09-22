package cafs

import (
	"crypto/sha256"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-ipfs/commands/files"
	"github.com/jbenet/go-base58"
	"github.com/multiformats/go-multihash"
	"io"
	"io/ioutil"
)

func NewMapstore() Filestore {
	return MapStore{}
}

// MapStore implements Filestore in-memory as a map. This thing needs attention.
// TODO - fixme
type MapStore map[datastore.Key][]byte

func (m MapStore) Put(data []byte, pin bool) (key datastore.Key, err error) {
	hash, err := hashBytes(data)
	if err != nil {
		return
	}

	key = datastore.NewKey("/map/" + hash)
	// set to the *original, non-byte* value
	m[key] = data
	return
}

func (m MapStore) Get(key datastore.Key) (value []byte, err error) {
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

// TODO - this will have to place nice with IPFS block hashing strategies
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

func (m MapStore) NewAdder(pin, wrap bool) (Adder, error) {
	addedOut := make(chan AddedFile, 8)
	done := make(chan bool, 0)

	return &adder{
		mapstore: m,
		out:      addedOut,
		done:     done,
	}, nil
}

// TODO - FINISH. THIS IMPLEMENTATION DOESN'T WORK.
// Adder wraps a coreunix adder to conform to the cafs adder interface
type adder struct {
	mapstore MapStore
	out      chan AddedFile
	done     chan bool
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
		data, err := ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		hash, err := hashBytes(data)
		if err != nil {
			return err
		}

		key := datastore.NewKey("/map/" + hash)
		// set to the *original, non-byte* value
		a.mapstore[key] = data
		a.out <- AddedFile{
			Name:  f.FileName(),
			Bytes: int64(len(data)),
			Hash:  hash,
		}
	}

	return nil
}

func (a *adder) Added() chan AddedFile {
	return a.out
}
func (a *adder) Close() error {
	a.done <- true
	return nil
}

package castore

import (
	"crypto/sha256"
	"github.com/ipfs/go-datastore"
	"github.com/jbenet/go-base58"
	"github.com/multiformats/go-multihash"
)

func NewMapstore() Datastore {
	return MapStore{}
}

// MapStore implements castore in-memory as a map
type MapStore map[datastore.Key][]byte

func (m MapStore) Put(data []byte) (key datastore.Key, err error) {
	hash, err := hashBytes(data)
	if err != nil {
		return
	}

	key = datastore.NewKey("/map/" + hash)
	// set to the *original* value
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

package cafs

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/ioutil"

	"github.com/ipfs/go-datastore"
	"github.com/jbenet/go-base58"
	"github.com/multiformats/go-multihash"
)

// NewMamstore allocates an instance of a mapstore
func NewMapstore() Filestore {
	return MapStore{}
}

// MapStore implements Filestore in-memory as a map

// An example pulled from tests will create a tree of "cafs"
// with directories & cafs, with paths properly set:
// NewMemdir("/a",
// 	NewMemfileBytes("a.txt", []byte("foo")),
// 	NewMemfileBytes("b.txt", []byte("bar")),
// 	NewMemdir("/c",
// 		NewMemfileBytes("d.txt", []byte("baz")),
// 		NewMemdir("/e",
// 			NewMemfileBytes("f.txt", []byte("bat")),
// 		),
// 	),
// )
// File is an interface that provides functionality for handling
// cafs/directories as values that can be supplied to commands.
//
// This is pretty close to things that already exist in ipfs
// and might not be necessary in most situations, but provides a sensible
// degree of modularity for our purposes:
// * memdir: github.com/ipfs/go-ipfs/commands/SerialFile
// * memfs: github.com/ipfs/go-ipfs/commands/ReaderFile
type MapStore map[datastore.Key]filer

func (m MapStore) PathPrefix() string {
	return "map"
}

func (m MapStore) Print() (string, error) {
	buf := &bytes.Buffer{}
	for key, file := range m {
		data, err := ioutil.ReadAll(file.File())
		if err != nil {
			return "", err
		}
		fmt.Fprintf(buf, "%s:%s\n\t%s\n", key, file.File().FileName(), string(data))
	}

	return buf.String(), nil
}

func (m MapStore) Put(file File, pin bool) (key datastore.Key, err error) {
	if file.IsDirectory() {
		buf := bytes.NewBuffer(nil)
		dir := fsDir{
			store: &m,
			path:  file.FullPath(),
			files: []datastore.Key{},
		}

		for {
			f, e := file.NextFile()
			if e != nil {
				if e.Error() == "EOF" {
					dirhash, e := hashBytes(buf.Bytes())
					if err != nil {
						err = fmt.Errorf("error hashing file data: %s", e.Error())
						return
					}
					// fmt.Println("dirhash:", dirhash)
					key = datastore.NewKey("/map/" + dirhash)
					m[key] = dir
					return

				}
				err = fmt.Errorf("error getting next file: %s", err.Error())
				return
			}

			hash, e := m.Put(f, pin)
			if e != nil {
				err = fmt.Errorf("error putting file: %s", e.Error())
				return
			}
			key = hash
			dir.files = append(dir.files, hash)
			_, err = buf.WriteString(key.String() + "\n")
			if err != nil {
				err = fmt.Errorf("error writing to buffer: %s", err.Error())
				return
			}
		}

	} else {
		data, e := ioutil.ReadAll(file)
		if e != nil {
			err = fmt.Errorf("error reading from file: %s", e.Error())
			return
		}
		hash, e := hashBytes(data)
		if e != nil {
			err = fmt.Errorf("error hashing file data: %s", e.Error())
			return
		}
		key = datastore.NewKey("/map/" + hash)
		m[key] = fsFile{name: file.FileName(), path: file.FullPath(), data: data}
		return
	}
	return
}

func (m MapStore) Get(key datastore.Key) (File, error) {
	if m[key] == nil {
		return nil, datastore.ErrNotFound
	}
	return m[key].File(), nil
}

func (m MapStore) Has(key datastore.Key) (exists bool, err error) {
	if m[key] == nil {
		return false, nil
	}
	return true, nil
}

func (m MapStore) Delete(key datastore.Key) error {
	delete(m, key)
	return nil
}

func (m MapStore) NewAdder(pin, wrap bool) (Adder, error) {
	addedOut := make(chan AddedFile, 9)
	return &adder{
		mapstore: m,
		out:      addedOut,
	}, nil
}

// Adder wraps a coreunix adder to conform to the cafs adder interface
type adder struct {
	mapstore MapStore
	pin      bool
	out      chan AddedFile
}

func (a *adder) AddFile(f File) error {
	path, err := a.mapstore.Put(f, a.pin)
	if err != nil {
		fmt.Errorf("error putting file in mapstore: %s", err.Error())
		return err
	}
	a.out <- AddedFile{
		Path:  path,
		Name:  f.FileName(),
		Bytes: 0,
		Hash:  path.String(),
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
		err = fmt.Errorf("error writing hash data: %s", err.Error())
		return
	}
	mhBuf, err := multihash.Encode(h.Sum(nil), multihash.SHA2_256)
	if err != nil {
		err = fmt.Errorf("error encoding hash: %s", err.Error())
		return
	}
	hash = base58.Encode(mhBuf)
	return
}

type fsFile struct {
	name string
	path string
	data []byte
}

func (f fsFile) File() File {
	return &Memfile{
		name: f.name,
		path: f.path,
		buf:  bytes.NewBuffer(f.data),
	}
}

type fsDir struct {
	store *MapStore
	path  string
	files []datastore.Key
}

func (f fsDir) File() File {
	files := make([]File, len(f.files))
	for i, path := range f.files {
		file, err := f.store.Get(path)
		if err != nil {
			panic(path.String())
		}
		files[i] = file
	}

	return &Memdir{
		path:  f.path,
		links: files,
	}
}

type filer interface {
	File() File
}

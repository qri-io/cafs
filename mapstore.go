package cafs

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/ipfs/go-datastore"
	"github.com/jbenet/go-base58"
	"github.com/multiformats/go-multihash"
)

// NewMapstore allocates an instance of a mapstore
func NewMapstore() *MapStore {
	return &MapStore{
		Network: make([]*MapStore, 0),
		Files:   make(map[datastore.Key]filer),
	}
}

// MapStore implements Filestore in-memory as a map
//
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
//
// Network simulates IPFS-like behavior, where nodes can connect
// to each other to retrieve data from other machines
type MapStore struct {
	Pinned  bool
	Network []*MapStore
	Files   map[datastore.Key]filer
}

// PathPrefix returns the prefix on paths in the store
func (m MapStore) PathPrefix() string {
	return "map"
}

// AddConnection sets up pointers from this MapStore to that, and vice versa.
func (m *MapStore) AddConnection(other *MapStore) {
	if other == m {
		return
	}
	// Add pointer from that network to this one.
	found := false
	for _, elem := range m.Network {
		if other == elem {
			found = true
		}
	}
	if !found {
		m.Network = append(m.Network, other)
	}
	// Add pointer from this network to that one.
	found = false
	for _, elem := range other.Network {
		if m == elem {
			found = true
		}
	}
	if !found {
		other.Network = append(other.Network, m)
	}
}

// Print converts the store to a string
func (m MapStore) Print() (string, error) {
	buf := &bytes.Buffer{}
	for key, file := range m.Files {
		data, err := ioutil.ReadAll(file.File())
		if err != nil {
			return "", err
		}
		fmt.Fprintf(buf, "%s:%s\n\t%s\n", key, file.File().FileName(), string(data))
	}

	return buf.String(), nil
}

// Put adds a file to the store
func (m *MapStore) Put(file File, pin bool) (key datastore.Key, err error) {
	if file.IsDirectory() {
		buf := bytes.NewBuffer(nil)
		dir := fsDir{
			store: m,
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
					m.Files[key] = dir
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
		m.Files[key] = fsFile{name: file.FileName(), path: file.FullPath(), data: data}
		return
	}
	return
}

// Get returns a File from the store
func (m *MapStore) Get(key datastore.Key) (File, error) {
	// key may be of the form /map/QmFoo/file.json but MapStore indexes its maps
	// using keys like /map/QmFoo. Trim after the second part of the key.
	parts := strings.Split(key.String(), "/")
	if len(parts) > 2 {
		prefix := strings.Join([]string{"", parts[1], parts[2]}, "/")
		key = datastore.NewKey(prefix)
	}
	// Check if the local MapStore has the file.
	f, err := m.getLocal(key)
	if err == nil {
		return f, nil
	} else if err != datastore.ErrNotFound {
		return nil, err
	}
	// Check if the anyone connected on the mock Network has the file.
	for _, connect := range m.Network {
		f, err := connect.getLocal(key)
		if err == nil {
			return f, nil
		} else if err != datastore.ErrNotFound {
			return nil, err
		}
	}
	return nil, datastore.ErrNotFound
}

func (m *MapStore) getLocal(key datastore.Key) (File, error) {
	if m.Files[key] == nil {
		return nil, datastore.ErrNotFound
	}
	return m.Files[key].File(), nil
}

// Has returns whether the store has a File with the key
func (m MapStore) Has(key datastore.Key) (exists bool, err error) {
	if m.Files[key] == nil {
		return false, nil
	}
	return true, nil
}

// Delete removes the file from the store with the key
func (m MapStore) Delete(key datastore.Key) error {
	delete(m.Files, key)
	return nil
}

// NewAdder returns an Adder for the store
func (m MapStore) NewAdder(pin, wrap bool) (Adder, error) {
	addedOut := make(chan AddedFile, 9)
	return &adder{
		mapstore: m,
		out:      addedOut,
	}, nil
}

var _ Fetcher = (*MapStore)(nil)
var _ Pinner = (*MapStore)(nil)

// Fetch returns a File from the store
func (m *MapStore) Fetch(source Source, key datastore.Key) (File, error) {
	// TODO: Perhaps Fetch should hit the network but Get should not?
	// Also, see comment in ./ipfs/filestore.go about local lists and integrating Fetch.
	if len(m.Network) == 0 {
		// TODO: Fetch only local files in this case. Fix test that depends on this.
		return nil, fmt.Errorf("this store cannot fetch from remote sources")
	}
	return m.Get(key)
}

// Pin pins a File with the given key
func (m *MapStore) Pin(key datastore.Key, recursive bool) error {
	if m.Pinned {
		return fmt.Errorf("already pinned")
	}
	m.Pinned = true
	return nil
}

// Unpin unpins a File with the given key
func (m *MapStore) Unpin(key datastore.Key, recursive bool) error {
	if !m.Pinned {
		return fmt.Errorf("not pinned")
	}
	m.Pinned = false
	return nil
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

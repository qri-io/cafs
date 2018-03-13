package gcloud

// import (
// 	"crypto/sha256"
// 	"fmt"
// 	"github.com/multiformats/go-multihash"

// 	"cloud.google.com/go/datastore"
// 	ipfsds "github.com/ipfs/go-datastore"
// 	"github.com/mr-tron/base58/base58"
// 	"github.com/qri-io/cafs"
// )

// type Store struct {
// 	client *datastore.Client
// }

// // NewMamstore allocates an instance of a mapstore
// func New() cafs.Filestore {
// 	return Store{}
// }

// // Put places a file or a directory in the store.
// // The most notable difference from a standard file store is the store itself determines
// // the resulting key (google "content addressing" for more info ;)
// // keys returned by put must be prefixed with the PathPrefix,
// // eg. /ipfs/QmZ3KfGaSrb3cnTriJbddCzG7hwQi2j6km7Xe7hVpnsW5S
// func (s *Store) Put(file cafs.File, pin bool) (key ipfsds.Key, err error) {
// 	if file.IsDirectory() {
// 		buf := bytes.NewBuffer(nil)
// 		dir := fsDir{
// 			store: &m,
// 			path:  file.FullPath(),
// 			files: []datastore.Key{},
// 		}

// 		for {
// 			f, e := file.NextFile()
// 			if e != nil {
// 				if e.Error() == "EOF" {
// 					dirhash, e := hashBytes(buf.Bytes())
// 					if err != nil {
// 						err = fmt.Errorf("error hashing file data: %s", e.Error())
// 						return
// 					}
// 					// fmt.Println("dirhash:", dirhash)
// 					key = datastore.NewKey("/map/" + dirhash)
// 					m[key] = dir
// 					return

// 				}
// 				err = fmt.Errorf("error getting next file: %s", err.Error())
// 				return
// 			}

// 			hash, e := m.Put(f, pin)
// 			if e != nil {
// 				err = fmt.Errorf("error putting file: %s", e.Error())
// 				return
// 			}
// 			key = hash
// 			dir.files = append(dir.files, hash)
// 			_, err = buf.WriteString(key.String() + "\n")
// 			if err != nil {
// 				err = fmt.Errorf("error writing to buffer: %s", err.Error())
// 				return
// 			}
// 		}

// 	} else {
// 		data, e := ioutil.ReadAll(file)
// 		if e != nil {
// 			err = fmt.Errorf("error reading from file: %s", e.Error())
// 			return
// 		}
// 		hash, e := hashBytes(data)
// 		if e != nil {
// 			err = fmt.Errorf("error hashing file data: %s", e.Error())
// 			return
// 		}
// 		key = datastore.NewKey("/map/" + hash)
// 		m[key] = fsFile{name: file.FileName(), path: file.FullPath(), data: data}
// 		return
// 	}
// 	return
// }

// // Get retrieves the object `value` named by `key`.
// // Get will return ErrNotFound if the key is not mapped to a value.
// func (s *Store) Get(key ipfsds.Key) (file cafs.File, err error) {
// 	if m[key] == nil {
// 		return nil, datastore.ErrNotFound
// 	}
// 	return m[key].File(), nil
// }

// // Has returns whether the `key` is mapped to a `value`.
// // In some contexts, it may be much cheaper only to check for existence of
// // a value, rather than retrieving the value itself. (e.g. HTTP HEAD).
// // The default implementation is found in `GetBackedHas`.
// func (s *Store) Has(key ipfsds.Key) (exists bool, err error) {
// 	if m[key] == nil {
// 		return false, nil
// 	}
// 	return true, nil
// }

// // Delete removes the value for given `key`.
// func (s *Store) Delete(key ipfsds.Key) error {
// 	return fmt.Errorf("not yet finished")
// }

// // NewAdder allocates an Adder instance for adding files to the filestore
// // Adder gives a higher degree of control over the file adding process at the
// // cost of being harder to work with.
// // "pin" is a flag for recursively pinning this object
// // "wrap" sets weather the top level should be wrapped in a directory
// // expect this to change to something like:
// // NewAdder(opt map[string]interface{}) (Adder, error)
// func (s *Store) NewAdder(pin, wrap bool) (cafs.Adder, error) {
// 	addedOut := make(chan cafs.AddedFile, 9)
// 	return &adder{
// 		mapstore: m,
// 		out:      addedOut,
// 	}, nil
// }

// // PathPrefix is a top-level identifier to distinguish between filestores,
// // for exmple: the "ipfs" in /ipfs/QmZ3KfGaSrb3cnTriJbddCzG7hwQi2j6km7Xe7hVpnsW5S
// // a Filestore implementation should always return the same value
// func (s *Store) PathPrefix() string {
// 	return "gcds"
// }

// // Fetch gets a file from a source
// // func (s *Store) Fetch(source Source, key ipfsds.Key) (cafs.SizeFile, error) {
// // }

// // address should return the base resource identifier in either content
// // or location based addressing schemes
// func (s *Store) Address() string {
// 	return "/gcds/"
// }

// func (m MapStore) NewAdder(pin, wrap bool) (cafs.Adder, error) {
// 	addedOut := make(chan cafs.AddedFile, 9)
// 	return &adder{
// 		mapstore: m,
// 		out:      addedOut,
// 	}, nil
// }

// // Adder wraps a coreunix adder to conform to the cafs adder interface
// type adder struct {
// 	mapstore MapStore
// 	pin      bool
// 	out      chan cafs.AddedFile
// }

// func (a *adder) AddFile(f cafs.File) error {
// 	path, err := a.mapstore.Put(f, a.pin)
// 	if err != nil {
// 		fmt.Errorf("error putting file in mapstore: %s", err.Error())
// 		return err
// 	}
// 	a.out <- cafs.AddedFile{
// 		Path:  path,
// 		Name:  f.FileName(),
// 		Bytes: 0,
// 		Hash:  path.String(),
// 	}
// 	return nil
// }

// func (a *adder) Added() chan cafs.AddedFile {
// 	return a.out
// }
// func (a *adder) Close() error {
// 	close(a.out)
// 	return nil
// }

// func hashBytes(data []byte) (hash string, err error) {
// 	mhBuf, err := multihash.Encode(sha256.Sum256(data)[:], multihash.SHA2_256)
// 	if err != nil {
// 		err = fmt.Errorf("error encoding hash: %s", err.Error())
// 		return
// 	}
// 	hash = base58.Encode(mhBuf)
// 	return
// }

// type fsFile struct {
// 	name string
// 	path string
// 	data []byte
// }

// func (f fsFile) File() cafs.File {
// 	return &Memfile{
// 		name: f.name,
// 		path: f.path,
// 		buf:  bytes.NewBuffer(f.data),
// 	}
// }

// type fsDir struct {
// 	store *MapStore
// 	path  string
// 	files []datastore.Key
// }

// func (f fsDir) File() cafs.File {
// 	files := make([]cafs.File, len(f.files))
// 	for i, path := range f.files {
// 		file, err := f.store.Get(path)
// 		if err != nil {
// 			panic(path.String())
// 		}
// 		files[i] = file
// 	}

// 	return &Memdir{
// 		path:     f.path,
// 		children: files,
// 	}
// }

// type filer interface {
// 	File() cafs.File
// }

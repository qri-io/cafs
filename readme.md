# cafs
--
    import "github.com/qri-io/cafs"

cafs is a "content-addressed-file-systen", which is a generalized interface for
working with content-addressed filestores. real-on-the-real, this is a wrapper
for IPFS. It looks a lot like the ipfs datastore interface, except the datastore
itself determines keys.

## Usage

#### type AddedFile

```go
type AddedFile struct {
	Path  datastore.Key
	Name  string
	Bytes int64
	Hash  string
	Size  string
}
```

AddedFile reports on the results of adding a file to the store TODO - add
filepath to this struct

#### type Adder

```go
type Adder interface {
	// AddFile adds a file or directory of files to the store
	// this function will return immideately, consumers should read
	// from the Added() channel to see the results of file addition.
	AddFile(files.File) error
	// Added gives a channel to read added files from.
	Added() chan AddedFile
	// In IPFS land close calls adder.Finalize() and adder.PinRoot()
	// (files will only be pinned if the pin flag was set on NewAdder)
	// Close will close the underlying
	Close() error
}
```

Adder is the interface for adding files to a Filestore. The addition process is
parallelized. Implementers must make all required AddFile calls, then call Close
to finalize the addition process. Progress can be monitored through the Added()
channel

#### type Filestore

```go
type Filestore interface {
	// put places a raw slice of bytes. Expect this to change to something like:
	// Put(file files.File, options map[string]interface{}) (key datastore.Key, err error)
	// The most notable difference from a standard file store is the store itself determines
	// the resulting key (google "content addressing" for more info ;)
	Put(file files.File, pin bool) (key datastore.Key, err error)

	// NewAdder should allocate an Adder instance for adding files to the filestore
	// Adder gives the highest degree of control over the file adding process at the
	// cost of being harder to work with.
	// "pin" is a flag for recursively pinning this object
	// "wrap" sets weather the top level should be wrapped in a directory
	// expect this to change to something like:
	// NewAdder(opt map[string]interface{}) (Adder, error)
	NewAdder(pin, wrap bool) (Adder, error)

	// Get retrieves the object `value` named by `key`.
	// Get will return ErrNotFound if the key is not mapped to a value.
	Get(key datastore.Key) (file files.File, err error)

	// Has returns whether the `key` is mapped to a `value`.
	// In some contexts, it may be much cheaper only to check for existence of
	// a value, rather than retrieving the value itself. (e.g. HTTP HEAD).
	// The default implementation is found in `GetBackedHas`.
	Has(key datastore.Key) (exists bool, err error)

	// Delete removes the value for given `key`.
	Delete(key datastore.Key) error
}
```

Filestore is an interface for working with a content-addressed file system. This
interface is under active development, expect it to change lots. It's currently
form-fitting around IPFS (ipfs.io), with far-off plans to generalize toward
compatibility with git (git-scm.com), then maybe other stuff, who knows.

#### func  NewMapstore

```go
func NewMapstore() Filestore
```

#### type MapStore

```go
type MapStore map[datastore.Key]files.File
```

MapStore implements Filestore in-memory as a map. This thing needs attention.
TODO - fixme

#### func (MapStore) Delete

```go
func (m MapStore) Delete(key datastore.Key) error
```

#### func (MapStore) Get

```go
func (m MapStore) Get(key datastore.Key) (files.File, error)
```

#### func (MapStore) Has

```go
func (m MapStore) Has(key datastore.Key) (exists bool, err error)
```

#### func (MapStore) NewAdder

```go
func (m MapStore) NewAdder(pin, wrap bool) (Adder, error)
```

#### func (MapStore) Put

```go
func (m MapStore) Put(file files.File, pin bool) (key datastore.Key, err error)
```

#### type Pinner

```go
type Pinner interface {
	Pin(key datastore.Key, recursive bool) error
	Unpin(key datastore.Key, recursive bool) error
}
```

TODO - This is an in-progress interface upgrade for content stores that support
the concept of pinning (originated by IPFS). *currently not in use, and not
implemented by anyone, ever*

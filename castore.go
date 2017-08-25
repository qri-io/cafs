// castore stands for "content-addressed store"
// it's intended to operate a sibling to the ipfs
// datastore interface, except the datastore itself
// determines keys for put operations
// More soon.
package castore

import (
	"github.com/ipfs/go-datastore"
)

type Datastore interface {
	// put determines what the key is
	Put(value []byte) (key datastore.Key, err error)

	// PutDir()

	// Get retrieves the object `value` named by `key`.
	// Get will return ErrNotFound if the key is not mapped to a value.
	Get(key datastore.Key) (value []byte, err error)

	// Has returns whether the `key` is mapped to a `value`.
	// In some contexts, it may be much cheaper only to check for existence of
	// a value, rather than retrieving the value itself. (e.g. HTTP HEAD).
	// The default implementation is found in `GetBackedHas`.
	Has(key datastore.Key) (exists bool, err error)

	// Delete removes the value for given `key`.
	Delete(key datastore.Key) error

	// Query searches the datastore and returns a query result. This function
	// may return before the query actually runs. To wait for the query:
	//
	//   result, _ := ds.Query(q)
	//
	//   // use the channel interface; result may come in at different times
	//   for entry := range result.Next() { ... }
	//
	//   // or wait for the query to be completely done
	//   entries, _ := result.Rest()
	//   for entry := range entries { ... }
	//
	// Query(q query.Query) (query.Results, error)
}

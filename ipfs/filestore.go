package ipfs_filestore

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"

	// goipld "github.com/ipfs/go-ipld-format"
	datastore "github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log"
	cafs "github.com/qri-io/cafs"

	// cbornode "gx/ipfs/QmNRz7BDWfdFNVLt7AVvmRefkrURD25EeoipcXqo6yoXU1/go-ipld-cbor"
	ipfsds "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore"
	// mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	blockservice "gx/ipfs/QmatUACvrFK3xYg1nd2iLAKfz7Yy5YB56tnzBYHpqiUuhn/go-ipfs/blockservice"
	core "gx/ipfs/QmatUACvrFK3xYg1nd2iLAKfz7Yy5YB56tnzBYHpqiUuhn/go-ipfs/core"
	coredag "gx/ipfs/QmatUACvrFK3xYg1nd2iLAKfz7Yy5YB56tnzBYHpqiUuhn/go-ipfs/core/coredag"
	corerepo "gx/ipfs/QmatUACvrFK3xYg1nd2iLAKfz7Yy5YB56tnzBYHpqiUuhn/go-ipfs/core/corerepo"
	coreunix "gx/ipfs/QmatUACvrFK3xYg1nd2iLAKfz7Yy5YB56tnzBYHpqiUuhn/go-ipfs/core/coreunix"
	dag "gx/ipfs/QmatUACvrFK3xYg1nd2iLAKfz7Yy5YB56tnzBYHpqiUuhn/go-ipfs/merkledag"
	path "gx/ipfs/QmatUACvrFK3xYg1nd2iLAKfz7Yy5YB56tnzBYHpqiUuhn/go-ipfs/path"
	ipfspin "gx/ipfs/QmatUACvrFK3xYg1nd2iLAKfz7Yy5YB56tnzBYHpqiUuhn/go-ipfs/pin"
	uarchive "gx/ipfs/QmatUACvrFK3xYg1nd2iLAKfz7Yy5YB56tnzBYHpqiUuhn/go-ipfs/unixfs/archive"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	// cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
	files "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit/files"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

var log = logging.Logger("cafs/ipfs")

type Filestore struct {
	node *core.IpfsNode
}

func (f Filestore) PathPrefix() string {
	return "ipfs"
}

func NewFilestore(config ...func(cfg *StoreCfg)) (*Filestore, error) {
	cfg := DefaultConfig()
	for _, option := range config {
		option(cfg)
	}

	if cfg.Node != nil {
		return &Filestore{
			node: cfg.Node,
		}, nil
	}

	if err := cfg.InitRepo(); err != nil {
		return nil, err
	}

	node, err := core.NewNode(cfg.Ctx, &cfg.BuildCfg)
	if err != nil {
		return nil, fmt.Errorf("error creating ipfs node: %s\n", err.Error())
	}

	return &Filestore{
		node: node,
	}, nil
}

func (fs *Filestore) Node() *core.IpfsNode {
	return fs.node
}

func (fs *Filestore) Has(key datastore.Key) (exists bool, err error) {
	ipfskey := ipfsds.NewKey(key.String())

	if _, err = core.Resolve(fs.node.Context(), fs.node.Namesys, fs.node.Resolver, path.Path(ipfskey.String())); err != nil {
		// TODO - return error here?
		return false, nil
	}

	return true, nil
}

func (fs *Filestore) Get(key datastore.Key) (cafs.File, error) {
	// fs.Node().Repo.Datastore().Get(key)
	return fs.getKey(key)
}

func (fs *Filestore) Fetch(source cafs.Source, key datastore.Key) (cafs.File, error) {
	return fs.getKey(key)
}

func (fs *Filestore) Put(file cafs.File, pin bool) (key datastore.Key, err error) {
	hash, err := fs.AddFile(file, pin)
	if err != nil {
		log.Infof("error adding bytes: %s", err.Error())
		return
	}
	return datastore.NewKey("/ipfs/" + hash), nil
}

func (fs *Filestore) Delete(path datastore.Key) error {
	// TODO - formally remove files?
	err := fs.Unpin(path, true)
	if err != nil {
		if err.Error() == "not pinned" {
			return nil
		}
	}
	return nil
}

func (fs *Filestore) getKey(key datastore.Key) (cafs.File, error) {
	p := path.Path(key.String())
	node := fs.node

	// TODO - we'll need a "local" list for this to work properly
	// currently this thing is *always* going to check the d.web for
	// a hash if it's online, which is a behaviour we need control over
	// might be worth expanding the cafs interface with the concept of
	// remote gets
	// update 2017-10-23 - we now have a fetch interface, integrate? is it integrated?
	dn, err := core.Resolve(node.Context(), node.Namesys, node.Resolver, p)
	if err != nil {
		return nil, fmt.Errorf("error resolving hash: %s", err.Error())
	}

	r, err := uarchive.DagArchive(node.Context(), dn, p.String(), node.DAG, false, 0)
	if err != nil {
		return nil, fmt.Errorf("error unarchiving DAG: %s", err.Error())
	}

	tr := tar.NewReader(r)

	// call next to set cursor at first file
	_, err = tr.Next()
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("tar archive error: %s", err.Error())
	}

	return cafs.NewMemfileReader(key.String(), tr), nil
}

// Adder wraps a coreunix adder to conform to the cafs adder interface
type Adder struct {
	adder *coreunix.Adder
	out   chan interface{}
	added chan cafs.AddedFile
}

func (a *Adder) AddFile(f cafs.File) error {
	return a.adder.AddFile(wrapFile{f})
}
func (a *Adder) Added() chan cafs.AddedFile {
	return a.added
}

func (a *Adder) Close() error {
	defer close(a.out)
	if _, err := a.adder.Finalize(); err != nil {
		return err
	}
	return a.adder.PinRoot()
}

func (fs *Filestore) NewAdder(pin, wrap bool) (cafs.Adder, error) {
	node := fs.Node()
	ctx := context.Background()
	bserv := blockservice.New(node.Blockstore, node.Exchange)
	dagserv := dag.NewDAGService(bserv)

	a, err := coreunix.NewAdder(ctx, node.Pinning, node.Blockstore, dagserv)
	if err != nil {
		return nil, fmt.Errorf("error allocating adder: %s", err.Error())
	}

	outChan := make(chan interface{}, 9)
	added := make(chan cafs.AddedFile, 9)
	a.Out = outChan
	a.Pin = pin
	a.Wrap = wrap

	go func() {
		for {
			select {
			case out, ok := <-outChan:
				if ok {
					output := out.(*coreunix.AddedObject)
					if len(output.Hash) > 0 {
						added <- cafs.AddedFile{
							Path:  datastore.NewKey("/ipfs/" + output.Hash),
							Name:  output.Name,
							Hash:  output.Hash,
							Bytes: output.Bytes,
							Size:  output.Size,
						}
					}
				} else {
					close(added)
					return
				}
			case <-ctx.Done():
				close(added)
				return
			}
		}
	}()

	return &Adder{
		adder: a,
		out:   outChan,
		added: added,
	}, nil
}

// AddAndPinBytes adds a file to the top level IPFS Node
func (fs *Filestore) AddFile(file cafs.File, pin bool) (hash string, err error) {
	node := fs.Node()

	ctx := context.Background()
	bserv := blockservice.New(node.Blockstore, node.Exchange)
	dagserv := dag.NewDAGService(bserv)

	fileAdder, err := coreunix.NewAdder(ctx, node.Pinning, node.Blockstore, dagserv)
	fileAdder.Pin = pin
	fileAdder.Wrap = file.IsDirectory()
	if err != nil {
		err = fmt.Errorf("error allocating adder: %s", err.Error())
		return
	}

	// wrap in a folder if top level is a file
	if !file.IsDirectory() {
		file = cafs.NewMemdir("/", file)
	}

	errChan := make(chan error, 0)
	outChan := make(chan interface{}, 9)

	fileAdder.Out = outChan

	go func() {
		defer close(outChan)
		for {
			file, err := file.NextFile()
			if err == io.EOF {
				// Finished the list of files.
				break
			} else if err != nil {
				errChan <- err
				return
			}
			if err := fileAdder.AddFile(wrapFile{file}); err != nil {
				errChan <- err
				return
			}
		}
		if _, err = fileAdder.Finalize(); err != nil {
			errChan <- fmt.Errorf("error finalizing file adder: %s", err.Error())
			return
		}
		if err = fileAdder.PinRoot(); err != nil {
			errChan <- fmt.Errorf("error pinning file root: %s", err.Error())
			return
		}
		// errChan <- nil
	}()

	for {
		select {
		case out, ok := <-outChan:
			if !ok {
				return
			}
			output := out.(*coreunix.AddedObject)
			if len(output.Hash) > 0 {
				hash = output.Hash
				// return
			}
		case err := <-errChan:
			return hash, err
		}

	}

	err = fmt.Errorf("something's gone horribly wrong")
	return
}

func (fs *Filestore) Pin(path datastore.Key, recursive bool) error {
	_, err := corerepo.Pin(fs.node, fs.node.Context(), []string{path.String()}, recursive)
	return err
}

func (fs *Filestore) Unpin(path datastore.Key, recursive bool) error {
	_, err := corerepo.Unpin(fs.node, fs.node.Context(), []string{path.String()}, recursive)
	return err
}

type wrapFile struct {
	cafs.File
}

func (w wrapFile) NextFile() (files.File, error) {
	next, err := w.File.NextFile()
	if err != nil {
		return nil, err
	}
	return wrapFile{next}, nil
}

func (fs *Filestore) DAGPut(f cafs.File, pin bool) (p string, err error) {
	n := fs.node
	dopin := pin

	// mhType tells inputParser which hash should be used. MaxUint64 means 'use
	// default hash' (sha256 for cbor, sha1 for git..)
	mhType := uint64(math.MaxUint64)

	outChan := make(chan *cid.Cid, 8)

	addAllAndPin := func(f files.File) error {
		cids := cid.NewSet()
		b := ipld.NewBatch(context.TODO(), n.DAG)

		for {
			file, err := f.NextFile()
			if err == io.EOF {
				// Finished the list of files.
				break
			} else if err != nil {
				return err
			}

			nds, err := coredag.ParseInputs("json", "cbor", file, mhType, -1)
			if err != nil {
				return err
			}
			if len(nds) == 0 {
				return fmt.Errorf("no node returned from ParseInputs")
			}

			for _, nd := range nds {
				err := b.Add(nd)
				if err != nil {
					return err
				}
			}

			cid := nds[0].Cid()
			cids.Add(cid)
			outChan <- cid
		}

		if err := b.Commit(); err != nil {
			return err
		}

		if dopin {
			defer n.Blockstore.PinLock().Unlock()

			cids.ForEach(func(c *cid.Cid) error {
				n.Pinning.PinWithMode(c, ipfspin.Recursive)
				return nil
			})

			err := n.Pinning.Flush()
			if err != nil {
				return err
			}
		}

		return nil
	}

	go func() {
		defer close(outChan)
		if !f.IsDirectory() {
			f = cafs.NewMemdir("/", f)
		}
		if err = addAllAndPin(wrapFile{f}); err != nil {
			return
		}
	}()

	for out := range outChan {
		p = out.String()
	}

	return
}

func (fs *Filestore) DAGGet(p string) (node cafs.DAGNode, err error) {
	var out interface{}
	n := fs.node

	dpath, e := path.ParsePath(p)
	if e != nil {
		return nil, e
	}

	obj, rem, e := n.Resolver.ResolveToLastNode(context.TODO(), dpath)
	if e != nil {
		return nil, e
	}

	out = obj
	if len(rem) > 0 {
		final, _, e := obj.Resolve(rem)
		if e != nil {
			return nil, e
		}
		out = rawNode{final}
	}

	if n, ok := out.(cafs.DAGNode); ok {
		return n, nil
	}

	return nil, fmt.Errorf("IPFS didn't return a valid cafs.DAGNode")
}

func (fs *Filestore) DAGDelete(path string) (err error) {
	return nil
}

type rawNode struct {
	value interface{}
}

func (rn rawNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(rn.value)
}

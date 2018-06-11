package ipfs_filestore

import (
	"archive/tar"
	"context"
	"fmt"
	"io"

	datastore "github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log"
	cafs "github.com/qri-io/cafs"

	ipfsds "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore"
	blockservice "gx/ipfs/QmatUACvrFK3xYg1nd2iLAKfz7Yy5YB56tnzBYHpqiUuhn/go-ipfs/blockservice"
	core "gx/ipfs/QmatUACvrFK3xYg1nd2iLAKfz7Yy5YB56tnzBYHpqiUuhn/go-ipfs/core"
	corerepo "gx/ipfs/QmatUACvrFK3xYg1nd2iLAKfz7Yy5YB56tnzBYHpqiUuhn/go-ipfs/core/corerepo"
	coreunix "gx/ipfs/QmatUACvrFK3xYg1nd2iLAKfz7Yy5YB56tnzBYHpqiUuhn/go-ipfs/core/coreunix"
	dag "gx/ipfs/QmatUACvrFK3xYg1nd2iLAKfz7Yy5YB56tnzBYHpqiUuhn/go-ipfs/merkledag"
	path "gx/ipfs/QmatUACvrFK3xYg1nd2iLAKfz7Yy5YB56tnzBYHpqiUuhn/go-ipfs/path"
	uarchive "gx/ipfs/QmatUACvrFK3xYg1nd2iLAKfz7Yy5YB56tnzBYHpqiUuhn/go-ipfs/unixfs/archive"
	files "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit/files"
)

var log = logging.Logger("cafs/ipfs")

type Filestore struct {
	cfg  *StoreCfg
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
		cfg:  cfg,
		node: node,
	}, nil
}

func (fs *Filestore) Node() *core.IpfsNode {
	return fs.node
}

func (fs *Filestore) Online() bool {
	return fs.node.OnlineMode()
}

func (fs *Filestore) GoOnline() error {
	cfg := fs.cfg
	cfg.BuildCfg.Online = true
	node, err := core.NewNode(cfg.Ctx, &cfg.BuildCfg)
	if err != nil {
		return fmt.Errorf("error creating ipfs node: %s\n", err.Error())
	}

	*fs = Filestore{
		cfg:  cfg,
		node: node,
	}
	return nil
}

func (fs *Filestore) Has(key datastore.Key) (exists bool, err error) {
	ipfskey := ipfsds.NewKey(key.String())
	// TODO - we'll need a "local" list for this to work properly
	// currently this thing is *always* going to check the d.web for
	// a hash if it's online, which is a behaviour we need control over
	// might be worth expanding the cafs interface with the concept of
	// remote gets
	// update 2017-10-23 - we now have a fetch interface, integrate? is it integrated?
	if _, err = core.Resolve(fs.node.Context(), fs.node.Namesys, fs.node.Resolver, path.Path(ipfskey.String())); err != nil {
		// TODO - return error here?
		return false, nil
	}

	// I had hoped this would work, it doesn't.
	// fs.Node().Repo.Datastore().Has(ipfskey)
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

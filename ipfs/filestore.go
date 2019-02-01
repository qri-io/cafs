package ipfs_filestore

import (
	"context"
	"fmt"
	"io"

	logging "github.com/ipfs/go-log"
	cafs "github.com/qri-io/cafs"
	"github.com/qri-io/fs"

	// Note coreunix is forked form github.com/ipfs/go-ipfs/core/coreunix
	// we need coreunix.Adder.addFile to be exported to get access to dags while
	// they're being created. We should be able to remove this with refactoring &
	// moving toward coreapi.coreUnix().Add() with properly-configured options,
	// but I'd like a test before we do that. We may also want to consider switching
	// Qri to writing IPLD. Lots to think about.
	coreunix "github.com/qri-io/cafs/ipfs/coreunix"

	path "gx/ipfs/QmT3rzed1ppXefourpmoZ7tyVQfsGPQZ1pHDngLmCvXxd3/go-path"
	core "gx/ipfs/QmUJYo4etAQqFfSS2rarFAE97eNGB8ej64YkRT2SmsYD4r/go-ipfs/core"
	coreapi "gx/ipfs/QmUJYo4etAQqFfSS2rarFAE97eNGB8ej64YkRT2SmsYD4r/go-ipfs/core/coreapi"
	coreiface "gx/ipfs/QmUJYo4etAQqFfSS2rarFAE97eNGB8ej64YkRT2SmsYD4r/go-ipfs/core/coreapi/interface"
	corerepo "gx/ipfs/QmUJYo4etAQqFfSS2rarFAE97eNGB8ej64YkRT2SmsYD4r/go-ipfs/core/corerepo"
	files "gx/ipfs/QmZMWMvWMVKCbHetJ4RgndbuEF1io2UpUxwQwtNjtYPzSC/go-ipfs-files"
	ipfsds "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
)

var log = logging.Logger("cafs/ipfs")

const prefix = "ipfs"

type Filestore struct {
	cfg  *StoreCfg
	node *core.IpfsNode
	capi coreiface.CoreAPI
}

func (fst Filestore) PathPrefix() string {
	return prefix
}

func NewFilestore(config ...Option) (*Filestore, error) {
	cfg := DefaultConfig()
	for _, option := range config {
		option(cfg)
	}

	if cfg.Node != nil {
		return &Filestore{
			cfg:  cfg,
			node: cfg.Node,
			capi: coreapi.NewCoreAPI(cfg.Node),
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
		capi: coreapi.NewCoreAPI(node),
	}, nil
}

func (fst *Filestore) Node() *core.IpfsNode {
	return fst.node
}

func (fst *Filestore) Online() bool {
	return fst.node.OnlineMode()
}

func (fst *Filestore) GoOnline() error {
	cfg := fst.cfg
	cfg.BuildCfg.Online = true
	node, err := core.NewNode(cfg.Ctx, &cfg.BuildCfg)
	if err != nil {
		return fmt.Errorf("error creating ipfs node: %s\n", err.Error())
	}

	*fst = Filestore{
		cfg:  cfg,
		node: node,
		capi: coreapi.NewCoreAPI(node),
	}

	if cfg.EnableAPI {
		go func() {
			if err := fst.serveAPI(); err != nil {
				log.Errorf("error serving IPFS HTTP api: %s", err)
			}
		}()
	}

	return nil
}

func (fst *Filestore) Has(key string) (exists bool, err error) {
	ipfskey := ipfsds.NewKey(key)

	if _, err = core.Resolve(fst.node.Context(), fst.node.Namesys, fst.node.Resolver, path.Path(ipfskey.String())); err != nil {
		// TODO - return error here?
		return false, nil
	}

	return true, nil
}

func (fst *Filestore) Get(key string) (fs.File, error) {
	return fst.getKey(key)
}

func (fst *Filestore) Fetch(source cafs.Source, key string) (fs.File, error) {
	return fst.getKey(key)
}

func (fst *Filestore) Put(file fs.File, pin bool) (key string, err error) {
	hash, err := fst.AddFile(file, pin)
	if err != nil {
		log.Infof("error adding bytes: %s", err.Error())
		return
	}
	return pathFromHash(hash), nil
}

func (fst *Filestore) Delete(key string) error {
	err := fst.Unpin(key, true)
	if err != nil {
		if err.Error() == "not pinned" {
			return nil
		}
	}
	return nil
}

func (fst *Filestore) getKey(key string) (fs.File, error) {
	path, err := coreiface.ParsePath(key)
	if err != nil {
		return nil, err
	}
	file, err := fst.capi.Unixfs().Get(fst.node.Context(), path)
	if err != nil {
		return nil, err
	}
	return fs.NewMemfileReader(file.FileName(), file), nil
}

// Adder wraps a coreunix adder to conform to the cafs adder interface
type Adder struct {
	adder *coreunix.Adder
	out   chan interface{}
	added chan cafs.AddedFile
}

func (a *Adder) AddFile(f fs.File) error {
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

func (fst *Filestore) NewAdder(pin, wrap bool) (cafs.Adder, error) {
	node := fst.node
	ctx := context.Background()

	a, err := coreunix.NewAdder(ctx, node.Pinning, node.Blockstore, node.DAG)
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
					output := out.(*coreiface.AddEvent)
					if len(output.Hash) > 0 {
						added <- cafs.AddedFile{
							Path:  pathFromHash(output.Hash),
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

func pathFromHash(hash string) string {
	return fmt.Sprintf("/%s/%s", prefix, hash)
}

// AddFile adds a file to the top level IPFS Node
func (fst *Filestore) AddFile(file fs.File, pin bool) (hash string, err error) {
	node := fst.Node()
	ctx := context.Background()

	fileAdder, err := coreunix.NewAdder(ctx, node.Pinning, node.Blockstore, node.DAG)
	fileAdder.Pin = pin
	fileAdder.Wrap = file.IsDirectory()
	if err != nil {
		err = fmt.Errorf("error allocating adder: %s", err.Error())
		return
	}

	// wrap in a folder if top level is a file
	if !file.IsDirectory() {
		file = fs.NewMemdir("/", file)
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
			output := out.(*coreiface.AddEvent)
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

func (fst *Filestore) Pin(path string, recursive bool) error {
	_, err := corerepo.Pin(fst.node, fst.capi, fst.node.Context(), []string{path}, recursive)
	return err
}

func (fst *Filestore) Unpin(path string, recursive bool) error {
	_, err := corerepo.Unpin(fst.node, fst.capi, fst.node.Context(), []string{path}, recursive)
	return err
}

type wrapFile struct {
	fs.File
}

func (w wrapFile) NextFile() (files.File, error) {
	next, err := w.File.NextFile()
	if err != nil {
		return nil, err
	}
	return wrapFile{next}, nil
}

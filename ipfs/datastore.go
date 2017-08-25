package ipfs_datastore

import (
	"context"
	"fmt"
	// "io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	// bstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	blockservice "github.com/ipfs/go-ipfs/blockservice"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreunix"
	dag "github.com/ipfs/go-ipfs/merkledag"
	// dagtest "github.com/ipfs/go-ipfs/merkledag/test"
	files "github.com/ipfs/go-ipfs/commands/files"
	path "github.com/ipfs/go-ipfs/path"
	// repo "github.com/ipfs/go-ipfs/repo"
	// fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
	tar "github.com/ipfs/go-ipfs/thirdparty/tar"
	uarchive "github.com/ipfs/go-ipfs/unixfs/archive"

	relds "github.com/ipfs/go-datastore"
	relquery "github.com/ipfs/go-datastore/query"
	// "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore"
)

type Datastore struct {
	// networkless ipfs node
	node *core.IpfsNode
}

func NewDatastore(config ...func(cfg *StoreCfg)) (*Datastore, error) {
	cfg := DefaultConfig()
	for _, c := range config {
		c(cfg)
	}

	if cfg.Node != nil {
		return &Datastore{
			node: cfg.Node,
		}, nil
	}

	if err := cfg.InitRepo(); err != nil {
		return nil, err
	}

	node, err := core.NewNode(cfg.Ctx, &cfg.BuildCfg)
	if err != nil {
		return nil, fmt.Errorf("error creating networkless ipfs node: %s\n", err.Error())
	}

	return &Datastore{
		node: node,
	}, nil
}

func (ds *Datastore) Has(key relds.Key) (exists bool, err error) {
	return false, fmt.Errorf("has is unsupported")
}

func (ds *Datastore) Get(key relds.Key) ([]byte, error) {
	return ds.getKey(key)
}

func (ds *Datastore) Put(data []byte) (key relds.Key, err error) {
	hash, err := ds.AddAndPinBytes(data)
	if err != nil {
		return relds.NewKey(""), err
	}
	return relds.NewKey("/ipfs/" + hash), nil
}

func (ds *Datastore) Delete(relds.Key) error {
	return fmt.Errorf("delete is unsupported")
}

func (ds *Datastore) Query(q relquery.Query) (relquery.Results, error) {
	return nil, fmt.Errorf("query is unsupported")
}

func (ds *Datastore) Batch() (relds.Batch, error) {
	return nil, relds.ErrBatchUnsupported
}

func (ds *Datastore) getKey(key relds.Key) ([]byte, error) {
	p := path.Path(key.String())
	node := ds.node
	dn, err := core.Resolve(node.Context(), node.Namesys, node.Resolver, p)
	if err != nil {
		fmt.Println("resolver error")
		return nil, err
	}

	// switch dn := dn.(type) {
	//   case *dag.ProtoNode:
	//     size, err := dn.Size()
	//     if err != nil {
	//       res.SetError(err, cmds.ErrNormal)
	//       return
	//     }

	//     res.SetLength(size)
	//   case *dag.RawNode:
	//     res.SetLength(uint64(len(dn.RawData())))
	//   default:
	//     res.SetError(fmt.Errorf("'ipfs get' only supports unixfs nodes"), cmds.ErrNormal)
	//     return
	//   }

	rdr, err := uarchive.DagArchive(node.Context(), dn, p.String(), node.DAG, false, 0)
	if err != nil {
		return nil, err
	}

	fp := filepath.Join("/tmp", key.BaseNamespace())

	e := tar.Extractor{
		Path:     fp,
		Progress: func(int64) int64 { return 0 },
	}
	if err := e.Extract(rdr); err != nil {
		return nil, err
	}

	return ioutil.ReadFile(fp)
}

func (ds *Datastore) AddAndPinPath(path string) (hash string, err error) {
	node := ds.node

	ctx := context.Background()
	bserv := blockservice.New(node.Blockstore, node.Exchange)
	dagserv := dag.NewDAGService(bserv)

	fileAdder, err := coreunix.NewAdder(ctx, node.Pinning, node.Blockstore, dagserv)
	if err != nil {
		return
	}

	fi, err := os.Stat(path)
	if err != nil {
		return
	}

	rfile, err := files.NewSerialFile("", path, false, fi)
	if err != nil {
		return
	}

	outChan := make(chan interface{}, 8)
	defer close(outChan)

	fileAdder.Out = outChan

	if err = fileAdder.AddFile(rfile); err != nil {
		return
	}

	if _, err = fileAdder.Finalize(); err != nil {
		return
	}

	if err = fileAdder.PinRoot(); err != nil {
		return
	}

	for {
		select {
		case out, ok := <-outChan:
			if ok {
				output := out.(*coreunix.AddedObject)
				if len(output.Hash) > 0 {
					hash = output.Hash
					return
				}
			}
		}
	}

	err = fmt.Errorf("something's gone horribly wrong")
	return
}

// AddAndPinBytes adds a file to the top level IPFS Node
func (ds *Datastore) AddAndPinBytes(data []byte) (hash string, err error) {
	node := ds.node

	ctx := context.Background()
	bserv := blockservice.New(node.Blockstore, node.Exchange)
	dagserv := dag.NewDAGService(bserv)

	fileAdder, err := coreunix.NewAdder(ctx, node.Pinning, node.Blockstore, dagserv)
	if err != nil {
		return
	}

	path := filepath.Join("/tmp", time.Now().String())

	if err = ioutil.WriteFile(path, data, os.ModePerm); err != nil {
		return
	}

	fi, err := os.Stat(path)
	if err != nil {
		return
	}

	rfile, err := files.NewSerialFile("", path, false, fi)
	if err != nil {
		return
	}

	outChan := make(chan interface{}, 8)
	defer close(outChan)

	fileAdder.Out = outChan

	if err = fileAdder.AddFile(rfile); err != nil {
		return
	}

	if _, err = fileAdder.Finalize(); err != nil {
		return
	}

	if err = fileAdder.PinRoot(); err != nil {
		return
	}

	for {
		select {
		case out, ok := <-outChan:
			if ok {
				output := out.(*coreunix.AddedObject)
				if len(output.Hash) > 0 {
					hash = output.Hash
					return
				}
			}
		}
	}

	err = fmt.Errorf("something's gone horribly wrong")
	return
}

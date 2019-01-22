package ipfs_filestore

import (
	"context"
	"fmt"

	"gx/ipfs/QmUJYo4etAQqFfSS2rarFAE97eNGB8ej64YkRT2SmsYD4r/go-ipfs/core"
	fsrepo "gx/ipfs/QmUJYo4etAQqFfSS2rarFAE97eNGB8ej64YkRT2SmsYD4r/go-ipfs/repo/fsrepo"
)

var ErrIPFSRepoNeedsMigration = fmt.Errorf(`Your IPFS repo needs an update!

Please make sure you have the latest version of IPFS (go-ipfs, at least v0.4.17)
from either https://dist.ipfs.io or your favourite package manager. 

Then run 'ipfs daemon' from a terminal. It should ask if you'd like to perform
a migration, select yes. 

Once you've migrated, run Qri connect again.`)

// StoreCfg configures the datastore
type StoreCfg struct {
	// embed options for creating a node
	core.BuildCfg
	// optionally just supply a node. will override everything
	Node *core.IpfsNode
	// path to a local filesystem fs repo
	FsRepoPath string
	// operating context
	Ctx context.Context
	// EnableAPI
	EnableAPI bool
}

// DefaultConfig results in a local node that
// attempts to draw from the default ipfs filesotre location
func DefaultConfig() *StoreCfg {
	return &StoreCfg{
		BuildCfg: core.BuildCfg{
			Online: false,
		},
		FsRepoPath: "~/.ipfs",
		Ctx:        context.Background(),
	}
}

// Option is a function that adjusts the store configuration
type Option func(o *StoreCfg)

// OptEnablePubSub configures ipfs to use the experimental pubsub store
func OptEnablePubSub(o *StoreCfg) {
	o.BuildCfg.ExtraOpts = map[string]bool{
		"pubsub": true,
	}
}

// OptsFromMap detects options from a map based on special keywords
func OptsFromMap(opts map[string]interface{}) Option {
	return func(o *StoreCfg) {
		if opts == nil {
			return
		}

		if api, ok := opts["api"].(bool); ok {
			o.EnableAPI = api
		}

		if ps, ok := opts["pubsub"].(bool); ok {
			if o.BuildCfg.ExtraOpts == nil {
				o.BuildCfg.ExtraOpts = map[string]bool{}
			}
			o.BuildCfg.ExtraOpts["pubsub"] = ps
		}

	}
}

func (cfg *StoreCfg) InitRepo() error {
	if cfg.NilRepo {
		return nil
	}
	if cfg.Repo != nil {
		return nil
	}
	if cfg.FsRepoPath != "" {
		localRepo, err := fsrepo.Open(cfg.FsRepoPath)
		if err != nil {
			if err == fsrepo.ErrNeedMigration {
				return ErrIPFSRepoNeedsMigration
			}
			return fmt.Errorf("error opening local filestore ipfs repository: %s\n", err.Error())
		}
		cfg.Repo = localRepo
	}
	return nil
}

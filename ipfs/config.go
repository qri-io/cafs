package ipfs_filestore

import (
	"context"
	"fmt"

	"gx/ipfs/QmViBzgruNUoLNBnXcx8YWbDNwV8MNGEGKkLo6JGetygdw/go-ipfs/core"
	fsrepo "gx/ipfs/QmViBzgruNUoLNBnXcx8YWbDNwV8MNGEGKkLo6JGetygdw/go-ipfs/repo/fsrepo"
)

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
}

// Default configuration results in a local node that
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
			return fmt.Errorf("error opening local filestore ipfs repository: %s\n", err.Error())
		}
		cfg.Repo = localRepo
	}
	return nil
}

package ipfs_filestore

import (
	"fmt"

	ipfs_config "gx/ipfs/QmPEpj17FDRpc7K1aArKZp3RsHtzRMKykeK9GVgn4WQGPR/go-ipfs-config"
	ipfs_commands "gx/ipfs/QmUJYo4etAQqFfSS2rarFAE97eNGB8ej64YkRT2SmsYD4r/go-ipfs/commands"
	ipfs_core "gx/ipfs/QmUJYo4etAQqFfSS2rarFAE97eNGB8ej64YkRT2SmsYD4r/go-ipfs/core"
	ipfs_corehttp "gx/ipfs/QmUJYo4etAQqFfSS2rarFAE97eNGB8ej64YkRT2SmsYD4r/go-ipfs/core/corehttp"
)

// serveAPI makes an IPFS node available over an HTTP api
func (fs *Filestore) serveAPI() error {
	if fs.node == nil {
		return fmt.Errorf("node is required to serve IPFS HTTP API")
	}

	cfg := fs.cfg
	addr := ""
	if cfg.Repo != nil {
		if ipfscfg, err := cfg.Repo.Config(); err == nil {
			// TODO (b5): apparantly ipfs config supports multiple API multiaddrs?
			// I dunno, for now just go with the most likely case of only assigning
			// an address if one string is supplied
			if len(ipfscfg.Addresses.API) == 1 {
				addr = ipfscfg.Addresses.API[0]
			}
		}
	}

	opts := []ipfs_corehttp.ServeOption{
		ipfs_corehttp.GatewayOption(true, "/ipfs", "/ipns"),
		ipfs_corehttp.WebUIOption,
		ipfs_corehttp.CommandsOption(cmdCtx(fs.node, cfg.FsRepoPath)),
	}

	// TODO (b5): I've added this fmt.Println because the corehttp package includes a println
	// call to the affect of "API server listening on [addr]", which will be confusing to our
	// users. We should chat with the protocol folks about making that print statement mutable
	// or configurable
	fmt.Println("starting IPFS HTTP API:")
	return ipfs_corehttp.ListenAndServe(fs.node, addr, opts...)
}

// extracted from github.com/ipfs/go-ipfs/cmd/ipfswatch/main.go
func cmdCtx(node *ipfs_core.IpfsNode, repoPath string) ipfs_commands.Context {
	return ipfs_commands.Context{
		Online:     true,
		ConfigRoot: repoPath,
		ReqLog:     &ipfs_commands.ReqLog{},
		LoadConfig: func(path string) (*ipfs_config.Config, error) {
			return node.Repo.Config()
		},
		ConstructNode: func() (*ipfs_core.IpfsNode, error) {
			return node, nil
		},
	}
}

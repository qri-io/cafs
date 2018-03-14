package ipfs_filestore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	assets "gx/ipfs/QmViBzgruNUoLNBnXcx8YWbDNwV8MNGEGKkLo6JGetygdw/go-ipfs/assets"
	core "gx/ipfs/QmViBzgruNUoLNBnXcx8YWbDNwV8MNGEGKkLo6JGetygdw/go-ipfs/core"
	namesys "gx/ipfs/QmViBzgruNUoLNBnXcx8YWbDNwV8MNGEGKkLo6JGetygdw/go-ipfs/namesys"
	config "gx/ipfs/QmViBzgruNUoLNBnXcx8YWbDNwV8MNGEGKkLo6JGetygdw/go-ipfs/repo/config"
	fsrepo "gx/ipfs/QmViBzgruNUoLNBnXcx8YWbDNwV8MNGEGKkLo6JGetygdw/go-ipfs/repo/fsrepo"
)

const (
	nBitsForKeypair = 2048
)

var errRepoExists = errors.New(`ipfs configuration file already exists!
Reinitializing would overwrite your keys.
`)

// InitRepo is a more specific version of the init command: github.com/ipfs/go-ipfs/cmd/ipfs/init.go
// it's adapted to let qri initialize a repo. This func should be maintained to reflect the
// ipfs master branch.
func InitRepo(repoPath, configPath string) error {
	if daemonLocked, err := fsrepo.LockedByOtherProcess(repoPath); err != nil {
		return err
	} else if daemonLocked {
		e := "ipfs daemon is running. please stop it to run this command"
		return fmt.Errorf(e)
	}

	var conf *config.Config
	if configPath != "" {
		confFile, err := os.Open(configPath)
		if err != nil {
			return fmt.Errorf("error opening configuration file: %s", err.Error())
		}
		conf = &config.Config{}
		if err := json.NewDecoder(confFile).Decode(conf); err != nil {
			// res.SetError(err, cmds.ErrNormal)
			return fmt.Errorf("invalid configuration file: %s", configPath)
		}
	}

	if err := doInit(ioutil.Discard, repoPath, false, nBitsForKeypair, nil, conf); err != nil {
		return err
	}

	return nil
}

func doInit(out io.Writer, repoRoot string, empty bool, nBitsForKeypair int, confProfiles []string, conf *config.Config) error {

	if err := checkWriteable(repoRoot); err != nil {
		return err
	}

	if fsrepo.IsInitialized(repoRoot) {
		return errRepoExists
	}
	if _, err := fmt.Fprintf(out, "initializing IPFS node at %s\n", repoRoot); err != nil {
		return err
	}

	if conf == nil {
		var err error
		conf, err = config.Init(out, nBitsForKeypair)
		if err != nil {
			return err
		}
	}

	for _, profile := range confProfiles {
		transformer, ok := config.ConfigProfiles[profile]
		if !ok {
			return fmt.Errorf("invalid configuration profile: %s", profile)
		}

		if err := transformer(conf); err != nil {
			return err
		}
	}

	if err := fsrepo.Init(repoRoot, conf); err != nil {
		return err
	}

	if !empty {
		if err := addDefaultAssets(out, repoRoot); err != nil {
			return err
		}
	}

	return initializeIpnsKeyspace(repoRoot)
}

func checkWriteable(dir string) error {
	_, err := os.Stat(dir)
	if err == nil {
		// dir exists, make sure we can write to it
		testfile := path.Join(dir, "test")
		fi, err := os.Create(testfile)
		if err != nil {
			if os.IsPermission(err) {
				return fmt.Errorf("%s is not writeable by the current user", dir)
			}
			return fmt.Errorf("unexpected error while checking writeablility of repo root: %s", err)
		}
		fi.Close()
		return os.Remove(testfile)
	}

	if os.IsNotExist(err) {
		// dir doesnt exist, check that we can create it
		return os.Mkdir(dir, 0775)
	}

	if os.IsPermission(err) {
		return fmt.Errorf("cannot write to %s, incorrect permissions", err)
	}

	return err
}

func addDefaultAssets(out io.Writer, repoRoot string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := fsrepo.Open(repoRoot)
	if err != nil { // NB: repo is owned by the node
		return err
	}

	nd, err := core.NewNode(ctx, &core.BuildCfg{Repo: r})
	if err != nil {
		return err
	}
	defer nd.Close()

	dkey, err := assets.SeedInitDocs(nd)
	if err != nil {
		return fmt.Errorf("init: seeding init docs failed: %s", err)
	}
	// log.Debugf("init: seeded init docs %s", dkey)

	if _, err = fmt.Fprintf(out, "to get started, enter:\n"); err != nil {
		return err
	}

	_, err = fmt.Fprintf(out, "\n\tipfs cat /ipfs/%s/readme\n\n", dkey)
	return err
}

func initializeIpnsKeyspace(repoRoot string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := fsrepo.Open(repoRoot)
	if err != nil { // NB: repo is owned by the node
		return err
	}

	nd, err := core.NewNode(ctx, &core.BuildCfg{Repo: r})
	if err != nil {
		return err
	}
	defer nd.Close()

	err = nd.SetupOfflineRouting()
	if err != nil {
		return err
	}

	return namesys.InitializeKeyspace(ctx, nd.DAG, nd.Namesys, nd.Pinning, nd.PrivateKey)
}

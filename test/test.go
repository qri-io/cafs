package test

import (
	"bytes"
	"fmt"
	"io/ioutil"

	"github.com/ipfs/go-datastore"
	"github.com/qri-io/cafs"
)

func EnsureFilestoreBehavior(f cafs.Filestore) error {
	if err := EnsureFilestoreSingleFileBehavior(f); err != nil {
		return err
	}
	if err := EnsureFilestoreAdderBehavior(f); err != nil {
		return err
	}
	return nil
}

func EnsureFilestoreSingleFileBehavior(f cafs.Filestore) error {
	fdata := []byte("foo")
	file := cafs.NewMemfileBytes("file.txt", fdata)
	key, err := f.Put(file, false)
	if err != nil {
		return fmt.Errorf("Filestore.Put(%s) error: %s", file.FileName(), err.Error())
	}

	pre := "/" + f.PathPrefix() + "/"
	if key.String()[:len(pre)] != pre {
		return fmt.Errorf("key returned didn't return a that matches this Filestore's PathPrefix. Expected: %s/..., got: %s", pre, key.String())
	}

	outf, err := f.Get(key)
	if err != nil {
		return fmt.Errorf("Filestore.Get(%s) error: %s", key.String(), err.Error())
	}
	data, err := ioutil.ReadAll(outf)
	if err != nil {
		return fmt.Errorf("error reading data from returned file: %s", err.Error())
	}
	if !bytes.Equal(fdata, data) {
		return fmt.Errorf("mismatched return value from get: %s != %s", string(fdata), string(data))
		// return fmt.Errorf("mismatched return value from get: %s != %s", outf.FileName(), string(data))
	}

	has, err := f.Has(datastore.NewKey("no-match"))
	if err != nil {
		return fmt.Errorf("Filestore.Has([nonexistent key]) error: %s", err.Error())
	}
	if has {
		return fmt.Errorf("filestore claims to have a very silly key")
	}

	// TODO - need to restore this, currently it'll make ipfs filestore tests fail
	has, err = f.Has(key)
	if err != nil {
		return fmt.Errorf("Filestore.Has(%s) error: %s", key.String(), err.Error())
	}
	if !has {
		return fmt.Errorf("Filestore.Has(%s) should have returned true", key.String())
	}
	if err = f.Delete(key); err != nil {
		return fmt.Errorf("Filestore.Delete(%s) error: %s", key.String(), err.Error())
	}

	return nil
}

func EnsureDirectoryBehavior(f cafs.Filestore) error {
	file := cafs.NewMemdir("/a",
		cafs.NewMemfileBytes("b.txt", []byte("a")),
		cafs.NewMemdir("c",
			cafs.NewMemfileBytes("d.txt", []byte("d")),
		),
		cafs.NewMemfileBytes("e.txt", []byte("e")),
	)
	key, err := f.Put(file, false)
	if err != nil {
		return fmt.Errorf("Filestore.Put(%s) error: %s", file.FileName(), err.Error())
	}

	outf, err := f.Get(key)
	if err != nil {
		return fmt.Errorf("Filestore.Get(%s) error: %s", key.String(), err.Error())
	}

	expectPaths := []string{
		"/a",
		"/a/b.txt",
		"/a/c",
		"/a/c/d.txt",
		"/a/e.txt",
	}

	paths := []string{}
	cafs.Walk(outf, 0, func(f cafs.File, depth int) error {
		paths = append(paths, f.FullPath())
		return nil
	})

	if len(paths) != len(expectPaths) {
		return fmt.Errorf("path length mismatch. expected: %d, got %d", len(expectPaths), len(paths))
	}

	for i, p := range expectPaths {
		if paths[i] != p {
			return fmt.Errorf("path %d mismatch expected: %s, got: %s", i, p, paths[i])
		}
	}

	if err = f.Delete(key); err != nil {
		return fmt.Errorf("Filestore.Delete(%s) error: %s", key.String(), err.Error())
	}

	return nil
}

func EnsureFilestoreAdderBehavior(f cafs.Filestore) error {
	adder, err := f.NewAdder(false, false)
	if err != nil {
		return fmt.Errorf("Filestore.NewAdder(false,false) error: %s", err.Error())
	}

	data := []byte("bar")
	if err := adder.AddFile(cafs.NewMemfileBytes("test.txt", data)); err != nil {
		return fmt.Errorf("Adder.AddFile error: %s", err.Error())
	}

	if err := adder.Close(); err != nil {
		return fmt.Errorf("Adder.Close() error: %s", err.Error())
	}

	return nil
}

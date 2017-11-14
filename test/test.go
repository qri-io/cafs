package test

import (
	"bytes"
	"fmt"
	"io/ioutil"

	"github.com/ipfs/go-datastore"
	"github.com/qri-io/cafs"
	"github.com/qri-io/cafs/memfs"
)

func RunFilestoreTests(f cafs.Filestore) error {
	fdata := []byte("foo")
	file := memfs.NewMemfileBytes("file.txt", fdata)
	key, err := f.Put(file, false)
	if err != nil {
		return fmt.Errorf("Filestore.Put(%s) error: %s", file.FileName(), err.Error())
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

	has, err := f.Has(datastore.NewKey("----------no-match---------"))
	if err != nil {
		return fmt.Errorf("Filestore.Has([nonexistent key]) error: %s", err.Error())
	}
	if has {
		return fmt.Errorf("filestore claims to have a very silly key")
	}

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

	if err := RunFilestoreAdderTests(f); err != nil {
		return err
	}

	return nil
}

func RunFilestoreAdderTests(f cafs.Filestore) error {
	adder, err := f.NewAdder(false, false)
	if err != nil {
		return fmt.Errorf("Filestore.NewAdder(false,false) error: %s", err.Error())
	}

	data := []byte("bar")
	if err := adder.AddFile(memfs.NewMemfileBytes("test.txt", data)); err != nil {
		return fmt.Errorf("Adder.AddFile error: %s", err.Error())
	}

	if err := adder.Close(); err != nil {
		return fmt.Errorf("Adder.Close() error: %s", err.Error())
	}

	return nil
}

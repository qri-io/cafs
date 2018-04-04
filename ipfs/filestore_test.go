package ipfs_filestore

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/qri-io/cafs"
	"github.com/qri-io/cafs/test"
)

var _ cafs.DAGStore = (*Filestore)(nil)

func newFilestore(name string) (f *Filestore, destroy func(), err error) {
	path := filepath.Join(os.TempDir(), name)

	err = os.MkdirAll(path, os.ModePerm)
	if err != nil {
		err = fmt.Errorf("error creating temp dir: %s", err.Error())
		return
	}

	destroy = func() {
		if err := os.RemoveAll(path); err != nil {
			fmt.Println(err.Error())
		}
	}

	err = InitRepo(path, "")
	if err != nil {
		err = fmt.Errorf("error intializing repo: %s", err.Error())
		return
	}

	f, err = NewFilestore(func(c *StoreCfg) {
		c.Online = false
		c.FsRepoPath = path
	})
	return
}

func TestFilestore(t *testing.T) {
	f, destroy, err := newFilestore("cafs_test_filestore")
	if err != nil {
		t.Error(err.Error())
		return
	}
	defer destroy()

	err = test.RunFilestoreTests(f)
	if err != nil {
		t.Errorf(err.Error())
	}
}

func TestDAG(t *testing.T) {
	f, destroy, err := newFilestore("cafs_test_dag")
	if err != nil {
		t.Error(err.Error())
		return
	}
	defer destroy()

	schemapath, err := f.DAGPut(cafs.NewMemfileBytes("schema.json", []byte(`{
      "type": "array",
      "items": {
        "type": "array",
        "items": [
          {
            "title": "city",
            "type": "string"
          },
          {
            "title": "pop",
            "type": "integer"
          },
          {
            "title": "avg_age",
            "type": "number"
          },
          {
            "title": "in_usa",
            "type": "boolean"
          }
        ]
      }
    }
  }`)), true)

	if err != nil {
		t.Errorf("error adding schema: %s", err.Error())
		return
	}
	t.Logf("schema.json: %s", schemapath)

	commitpath, err := f.DAGPut(cafs.NewMemfileBytes("commit.json", []byte(`{
    "qri": "cm:0",
    "title": "initial commit"
  }`)), true)

	if err != nil {
		t.Errorf("error adding schema: %s", err.Error())
		return
	}
	t.Logf("commit.json: %s", commitpath)

	data := fmt.Sprintf(`{
  "meta": {
    "qri": "md:0",
    "title": "example city data"
  },
  "commit": { "/" : "%s" },
  "qri": "ds:0",
  "structure": {
    "qri": "st:0",
    "format": "csv",
    "formatConfig": {
      "headerRow": true
    },
    "schema": { "/" : "%s" }
  },
  "visconfig": {
    "qri": "vc:0",
    "format": "example format",
    "visualizations": "example visualization"
  }
}`, commitpath, schemapath)

	path, err := f.DAGPut(cafs.NewMemfileBytes("dataset.json", []byte(data)), true)
	if err != nil {
		t.Error(err.Error())
		return
	}
	t.Log(path)

	node, err := f.DAGGet(path)
	if err != nil {
		t.Error(err.Error())
		return
	}

	d, err := node.MarshalJSON()
	if err != nil {
		t.Errorf("DAGNode unmarshalJSON failed: %s", err.Error())
		return
	}
	t.Logf("%s", d)

	node, err = f.DAGGet(fmt.Sprintf("%s/structure/schema/type", path))
	if err != nil {
		t.Error(err.Error())
		return
	}

	d, err = node.MarshalJSON()
	if err != nil {
		t.Errorf("DAGNode unmarshalJSON failed: %s", err.Error())
		return
	}
	t.Logf("%s", d)
	// file, err := f.Get(datastore.NewKey(d.(p)))
	// if err != nil {
	// 	t.Error(err.Error())
	// 	return
	// }

	// outdata, err := ioutil.ReadAll(file)
	// if err != nil {
	// 	t.Error(err.Error())
	// 	return
	// }

	// t.Log(string(outdata))

	// if file.FileName() != "dataset" {
	// 	t.Errorf("expected filename to be dataset.json, got: %s", file)
	// 	return
	// }

	err = f.DAGDelete(path)
	if err != nil {
		t.Errorf("error deleting: %s", err.Error())
		return
	}

}

func BenchmarkRead(b *testing.B) {
	f, destroy, err := newFilestore("cafs_test_benchmark")
	if err != nil {
		b.Error(err.Error())
		return
	}
	defer destroy()

	egFilePath := "testdata/complete.json"
	data, err := ioutil.ReadFile(egFilePath)
	if err != nil {
		b.Errorf("error reading temp file data: %s", err.Error())
		return
	}

	key, err := f.Put(cafs.NewMemfileBytes(filepath.Base(egFilePath), data), true)
	if err != nil {
		b.Errorf("error putting example file in store: %s", err.Error())
		return
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gotf, err := f.Get(key)
		if err != nil {
			b.Errorf("iteration %d error getting key: %s", i, err.Error())
			break
		}

		gotData, err := ioutil.ReadAll(gotf)
		if err != nil {
			b.Errorf("iteration %d error reading data bytes: %s", i, err.Error())
			break
		}

		if len(data) != len(gotData) {
			b.Errorf("iteration %d byte length mistmatch. expected: %d, got: %d", i, len(data), len(gotData))
			break
		}

		un := map[string]interface{}{}
		if err := json.Unmarshal(gotData, &un); err != nil {
			b.Errorf("iteration %d error unmarshaling data: %s", i, err.Error())
			break
		}
	}
}

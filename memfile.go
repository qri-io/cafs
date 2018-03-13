package cafs

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
)

// PathSetter adds the capacity to modify a path property
type PathSetter interface {
	SetPath(path string)
}

// Memfile is an in-memory file
type Memfile struct {
	buf  io.Reader
	name string
	path string
}

// Confirm that Memfile satisfies the File interface
var _ = (File)(&Memfile{})

// NewMemfileBytes creates a file from an io.Reader
func NewMemfileReader(name string, r io.Reader) *Memfile {
	return &Memfile{
		buf:  r,
		name: name,
	}
}

// NewMemfileBytes creates a file from a byte slice
func NewMemfileBytes(name string, data []byte) *Memfile {
	return &Memfile{
		buf:  bytes.NewBuffer(data),
		name: name,
	}
}

func (m Memfile) Read(p []byte) (int, error) {
	return m.buf.Read(p)
}

func (m Memfile) Close() error {
	if closer, ok := m.buf.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (m Memfile) FileName() string {
	return m.name
}

func (m Memfile) FullPath() string {
	return m.path
}

func (m *Memfile) SetPath(path string) {
	m.path = path
}

func (Memfile) IsDirectory() bool {
	return false
}

func (Memfile) NextFile() (File, error) {
	return nil, ErrNotDirectory
}

// Memdir is an in-memory directory
// Currently it only supports either Memfile & Memdir as links
type Memdir struct {
	path  string
	fi    int // file index for reading
	links []File
}

// Confirm that Memdir satisfies the File interface
var _ = (File)(&Memdir{})

// NewMemdir creates a new Memdir, supplying zero or more links
func NewMemdir(path string, links ...File) *Memdir {
	m := &Memdir{
		path:  path,
		links: []File{},
	}
	m.AddChildren(links...)
	return m
}

func (Memdir) Close() error {
	return ErrNotReader
}

func (Memdir) Read([]byte) (int, error) {
	return 0, ErrNotReader
}

func (m Memdir) FileName() string {
	return filepath.Base(m.path)
}

func (m Memdir) FullPath() string {
	return m.path
}

func (Memdir) IsDirectory() bool {
	return true
}

func (d *Memdir) NextFile() (File, error) {
	if d.fi >= len(d.links) {
		d.fi = 0
		return nil, io.EOF
	}
	defer func() { d.fi++ }()
	return d.links[d.fi], nil
}

func (d *Memdir) SetPath(path string) {
	d.path = path
	for _, f := range d.links {
		if fps, ok := f.(PathSetter); ok {
			fps.SetPath(filepath.Join(path, f.FileName()))
		}
	}
}

// AddChildren allows any sort of file to be added, but only
// implementations that implement the PathSetter interface will have
// properly configured paths.
func (d *Memdir) AddChildren(fs ...File) {
	for _, f := range fs {
		if fps, ok := f.(PathSetter); ok {
			fps.SetPath(filepath.Join(d.FullPath(), f.FileName()))
		}
		dir := d.MakeDirP(f)
		dir.links = append(dir.links, f)
	}
}

func (d *Memdir) ChildDir(dirname string) *Memdir {
	if dirname == "" || dirname == "." || dirname == "/" {
		return d
	}
	for _, f := range d.links {
		if dir, ok := f.(*Memdir); ok {
			if filepath.Base(dir.path) == dirname {
				return dir
			}
		}
	}
	return nil
}

func (d *Memdir) MakeDirP(f File) *Memdir {
	dirpath, _ := filepath.Split(f.FileName())
	if dirpath == "" {
		return d
	}
	dirs := strings.Split(dirpath[1:len(dirpath)-1], "/")
	if len(dirs) == 1 {
		return d
	}

	dir := d
	for _, dirname := range dirs {
		if ch := dir.ChildDir(dirname); ch != nil {
			dir = ch
			continue
		}
		ch := NewMemdir(filepath.Join(dir.FullPath(), dirname))
		dir.links = append(dir.links, ch)
		dir = ch
	}
	return dir
}

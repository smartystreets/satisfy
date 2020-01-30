package core

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type inMemoryFileSystem struct {
	fileSystem map[string]*file
	Root       string
}

func newInMemoryFileSystem() *inMemoryFileSystem {
	return &inMemoryFileSystem{
		fileSystem: make(map[string]*file),
	}
}

func (this *inMemoryFileSystem) Stat(path string) (contracts.FileInfo, error) {
	file, found := this.fileSystem[path]
	if found {
		return file, nil
	} else {
		return file, os.ErrNotExist
	}
}

func (this *inMemoryFileSystem) Listing() (files []contracts.FileInfo) {
	for _, file := range this.fileSystem {
		files = append(files, file)
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Path() < files[j].Path() })
	return files
}

func (this *inMemoryFileSystem) Open(path string) io.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader(this.fileSystem[path].contents))
}

func (this *inMemoryFileSystem) Create(path string) io.WriteCloser {
	this.WriteFile(path, nil)
	return this.fileSystem[path]
}

func (this *inMemoryFileSystem) ReadFile(path string) []byte {
	target := this.fileSystem[path]
	if target.symlink != "" {
		target = this.resolveSymlink(target)

	}
	return target.contents
}

func (this *inMemoryFileSystem) resolveSymlink(target *file) *file {
	source, found := this.fileSystem[target.symlink]
	if found {
		return source
	}
	parts := strings.Split(target.path, string(os.PathSeparator))
	for part := 1; part < len(parts); part++ {
		prepend := filepath.Join(parts[:part]...)
		path := filepath.Join(prepend, target.symlink)
		source, found := this.fileSystem[path]
		if found {
			return source
		}
	}
	return nil
}

func (this *inMemoryFileSystem) WriteFile(path string, content []byte) {
	this.fileSystem[path] = &file{
		path:     path,
		contents: content,
		mod:      InMemoryModTime,
	}
}

func (this *inMemoryFileSystem) CreateSymlink(source, target string) {
	this.fileSystem[target] = &file{
		path:     target,
		contents: nil,
		mod:      InMemoryModTime,
		symlink:  source,
	}
}

func (this *inMemoryFileSystem) Delete(path string) {
	this.fileSystem[path] = nil
	delete(this.fileSystem, path)
}

func (this *inMemoryFileSystem) RootPath() string {
	return this.Root
}

/////////////////////////////////////////////////

type file struct {
	path     string
	contents []byte
	mod      time.Time
	symlink  string
}

func (this *file) Symlink() string { return this.symlink }

var InMemoryModTime = time.Now()

func (this *file) ModTime() time.Time {
	return this.mod
}

func (this *file) Write(p []byte) (n int, err error) {
	this.contents = append(this.contents, p...)
	return len(p), nil
}

func (this *file) Close() error {
	return nil
}

func (this *file) Path() string {
	return this.path
}

func (this *file) Size() int64 {
	return int64(len(this.contents))
}
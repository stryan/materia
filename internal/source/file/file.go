package file

import (
	"context"
	"os"
)

type FileSource struct {
	path string
}

func (f *FileSource) Close(_ context.Context) (_ error) {
	return nil
}

func (f *FileSource) Clean() (_ error) {
	return os.RemoveAll(f.path)
}

func NewFileSource(path string) *FileSource {
	return &FileSource{path}
}

func (f *FileSource) Sync(ctx context.Context) error {
	return nil
}

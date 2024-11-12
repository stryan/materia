package file

import (
	"context"
	"os"

	"github.com/charmbracelet/log"
)

type FileSource struct {
	repo string
	path string
}

func (f *FileSource) Close(_ context.Context) (_ error) {
	return nil
}

func (f *FileSource) Clean() (_ error) {
	return os.RemoveAll(f.path)
}

func NewFileSource(path, repo string) *FileSource {
	log.Info("file source", "path", path, "repo", repo)
	return &FileSource{repo, path}
}

func (f *FileSource) Sync(ctx context.Context) error {
	if err := os.RemoveAll(f.path); err != nil {
		return err
	}
	repoFS := os.DirFS(f.repo)
	return os.CopyFS(f.path, repoFS)
}

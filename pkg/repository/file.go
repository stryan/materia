package repository

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

type FileRepository struct {
	Prefix string
}

func NewFileRepository(prefix string) (*FileRepository, error) {
	if _, err := os.Stat(prefix); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = os.Mkdir(prefix, 0o755)
			if err != nil {
				return nil, fmt.Errorf("error creating FileRepository with prefix %v: %w", prefix, err)
			}
		}
	}
	return &FileRepository{prefix}, nil
}

func (filerepository *FileRepository) Install(ctx context.Context, path string, data []byte) error {
	err := os.WriteFile(filepath.Join(filerepository.Prefix, path), data, 0o755)
	if err != nil {
		return err
	}
	return nil
}

func (filerepository *FileRepository) Remove(ctx context.Context, path string) error {
	err := os.Remove(filepath.Join(filerepository.Prefix, path))
	if err != nil {
		return err
	}
	return nil
}

func (filerepository *FileRepository) Exists(ctx context.Context, path string) (bool, error) {
	_, err := os.Stat(filepath.Join(filerepository.Prefix, path))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return true, nil
}

func (filerepository *FileRepository) Get(ctx context.Context, path string) (string, error) {
	return filepath.Join(filerepository.Prefix, path), nil
}

func (filerepository *FileRepository) List(ctx context.Context) ([]string, error) {
	panic("unimplemented")
}

func (filerepository *FileRepository) Clean(ctx context.Context) error {
	entries, err := os.ReadDir(filerepository.Prefix)
	if err != nil {
		return err
	}
	for _, v := range entries {
		err := os.RemoveAll(filepath.Join(filerepository.Prefix, v.Name()))
		if err != nil {
			return err
		}
	}
	return nil
}

func atomicWrite(root *os.Root, path string, mode os.FileMode, content []byte) (err error) {
	// make sure the parent actually exists first
	dir := filepath.Dir(path)
	if dir != "." {
		if err := root.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	var suffix [6]byte
	_, _ = rand.Read(suffix[:]) // or crypto.Text and sliced down
	tmp := filepath.Join(dir, fmt.Sprintf(".%s.%x.materia_tmp", filepath.Base(path), suffix))

	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = root.Remove(tmp)
		}
	}()

	if _, err = f.Write(content); err != nil {
		return err
	}
	if err = f.Chmod(mode); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return root.Rename(tmp, path)
}

var tmpFileRegex = regexp.MustCompile(`^\..+\.[0-9a-f]{12}\.materia_tmp$`)

func cleanupAtomicTemps(root *os.Root) error {
	var errs []error
	rfs := root.FS()

	err := fs.WalkDir(rfs, ".", func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() || !tmpFileRegex.MatchString(e.Name()) {
			return nil
		}

		// we've got ourselves a temp file
		err = root.Remove(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// problem solved
				return nil
			} else {
				errs = append(errs, err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return errors.Join(append(errs, err)...)
}

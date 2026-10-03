package repository

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

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
			}
			errs = append(errs, err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return errors.Join(append(errs, err)...)
}

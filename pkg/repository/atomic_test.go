package repository

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAtomicWrite(t *testing.T) {
	root := func(t *testing.T) *os.Root {
		r, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		t.Cleanup(func() { _ = r.Close() })
		return r
	}

	t.Run("smoketest", func(t *testing.T) {
		r := root(t)
		require.NoError(t, atomicWrite(r, "foo.txt", 0o755, []byte("hi")))
		assert.Equal(t, os.FileMode(0o755), perm(t, filepath.Join(r.Name(), "foo.txt")))
		res, err := r.ReadFile("foo.txt")
		require.NoError(t, err)
		assert.Equal(t, []byte("hi"), res)
	})

	t.Run("mode ignores umask", func(t *testing.T) {
		old := syscall.Umask(0o077)
		t.Cleanup(func() { syscall.Umask(old) })
		r := root(t)
		require.NoError(t, atomicWrite(r, "x.sh", 0o755, []byte("hi")))
		assert.Equal(t, os.FileMode(0o755), perm(t, filepath.Join(r.Name(), "x.sh")))
	})

	t.Run("update", func(t *testing.T) {
		r := root(t)
		path := writeFile(t, r.Name(), "test.txt", "hello world", 0o755)
		rel, err := filepath.Rel(r.Name(), path)
		require.NoError(t, err)
		require.NoError(t, atomicWrite(r, rel, 0o644, []byte("hello")))
		res, err := r.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, []byte("hello"), res)
		assert.Equal(t, os.FileMode(0o644), perm(t, path))
	})
}

func TestAtomicWrite_FailedRenameLeavesNoTemp(t *testing.T) {
	r, err := os.OpenRoot(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })
	require.NoError(t, r.Mkdir("foo", 0o755))
	require.NoError(t, r.WriteFile("foo/test.txt", []byte("hello"), 0o644))
	require.Error(t, atomicWrite(r, "foo", 0o644, []byte("wrong")))
}

func Test_CleanupAtomicTemps(t *testing.T) {
	r, err := os.OpenRoot(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })
	require.NoError(t, r.Mkdir("bar", 0o755))
	require.NoError(t, r.WriteFile(".foo.0123456789ab.materia_tmp", []byte{}, 0o644))
	require.NoError(t, r.WriteFile("bar/.foo.0123456789ab.materia_tmp", []byte{}, 0o644))
	require.NoError(t, r.WriteFile(".x.tmp", []byte{}, 0o644))
	require.NoError(t, cleanupAtomicTemps(r))
	_, err = r.Stat(".foo.0123456789ab.materia_tmp")
	require.ErrorIs(t, err, fs.ErrNotExist)
	_, err = r.Stat("bar/.foo.0123456789ab.materia_tmp")
	require.ErrorIs(t, err, fs.ErrNotExist)
	_, err = r.Stat(".x.tmp")
	require.NoError(t, err)
}

func perm(t *testing.T, p string) os.FileMode {
	t.Helper()
	info, err := os.Lstat(p)
	require.NoError(t, err)
	return info.Mode().Perm()
}

func writeFile(t *testing.T, root, rel, content string, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(content), mode))
	require.NoError(t, os.Chmod(p, mode))
	return p
}

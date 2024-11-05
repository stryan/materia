package git

import (
	"context"
	"errors"
	"os"

	"github.com/go-git/go-git/v5"
)

type GitSource struct {
	repo string
	path string
}

func NewGitSource(path, repo string) *GitSource {
	return &GitSource{
		repo: repo,
		path: path,
	}
}

func (g *GitSource) Sync(ctx context.Context) error {
	_, err := git.PlainCloneContext(ctx, g.path, false, &git.CloneOptions{
		URL:      g.repo,
		Progress: os.Stdout,
	})
	if err != nil && !errors.Is(err, git.ErrRepositoryAlreadyExists) {
		return err
	}
	if errors.Is(err, git.ErrRepositoryAlreadyExists) {
		r, err := git.PlainOpen(g.path)
		if err != nil {
			return err
		}
		w, err := r.Worktree()
		if err != nil {
			return err
		}
		err = w.PullContext(ctx, &git.PullOptions{})
		if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return err
		}
	}

	return nil
}

func (g *GitSource) Close(ctx context.Context) error {
	return nil
}

func (g *GitSource) Clean() (_ error) {
	return os.RemoveAll(g.path)
}

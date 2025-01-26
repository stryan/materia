package git

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

type GitSource struct {
	repo       string
	path       string
	privateKey string
}

func NewGitSource(path, repo, priv string) *GitSource {
	return &GitSource{
		repo:       repo,
		path:       path,
		privateKey: priv,
	}
}

func (g *GitSource) Sync(ctx context.Context) error {
	var options git.CloneOptions
	var pullOptions git.PullOptions
	if g.privateKey != "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		_, err = os.Stat(fmt.Sprintf("%v/.ssh/known_hosts", home))
		if err != nil {
			return err
		}
		_, err = os.Stat(g.privateKey)
		if err != nil {
			return err
		}
		publicKeys, err := ssh.NewPublicKeysFromFile("git", g.privateKey, "")
		if err != nil {
			return err
		}
		options.Auth = publicKeys
		pullOptions.Auth = publicKeys
	}
	options.URL = g.repo
	options.Progress = os.Stdout
	_, err := git.PlainCloneContext(ctx, g.path, false, &options)
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
		err = w.PullContext(ctx, &pullOptions)
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

package git

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	xssh "golang.org/x/crypto/ssh"
)

type GitSource struct {
	repo     string
	path     string
	auth     transport.AuthMethod
	insecure bool
}

func NewGitSource(path, repo string, c *Config) (*GitSource, error) {
	g := &GitSource{
		repo: repo,
		path: path,
	}
	if c != nil {
		if c.PrivateKey != "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			_, err = os.Stat(fmt.Sprintf("%v/.ssh/known_hosts", home))
			if err != nil {
				return nil, err
			}
			_, err = os.Stat(c.PrivateKey)
			if err != nil {
				return nil, err
			}
			publicKeys, err := ssh.NewPublicKeysFromFile("git", c.PrivateKey, "")
			if err != nil {
				return nil, err
			}
			if g.insecure {
				publicKeys.HostKeyCallback = xssh.InsecureIgnoreHostKey()
			}
			g.auth = publicKeys
		} else if c.Username != "" {
			g.auth = &http.BasicAuth{
				Username: c.Username,
				Password: c.Password,
			}
		} else {
			return nil, errors.New("no valid authentication set for git")
		}
	}
	return g, nil
}

func (g *GitSource) Sync(ctx context.Context) error {
	var options git.CloneOptions
	var pullOptions git.PullOptions

	options.URL = g.repo
	options.Progress = os.Stdout
	options.Auth = g.auth
	pullOptions.Auth = g.auth
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

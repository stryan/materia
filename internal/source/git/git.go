package git

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	xssh "golang.org/x/crypto/ssh"
)

type GitSource struct {
	repo     string
	branch   string
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
		g.branch = c.Branch
		if c.PrivateKey != "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			hostsfile := c.KnownHosts
			if hostsfile == "" {
				hostsfile = fmt.Sprintf("%v/.ssh/known_hosts", home)
			}
			_, err = os.Stat(hostsfile)
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
	localPath := g.path
	localBranch := g.branch
	remoteBranch := fmt.Sprintf("origin/%v", g.branch)
	repoURL := g.repo
	stale := false
	// Clone the repository
	r, err := git.PlainClone(localPath, false, &git.CloneOptions{
		URL:               repoURL,
		Auth:              g.auth,
		Progress:          os.Stdout,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	})
	if errors.Is(err, git.ErrRepositoryAlreadyExists) {
		r, err = git.PlainOpen(localPath)
		if err != nil {
			log.Fatal(err)
		}
		stale = true
	}
	if err != nil && !errors.Is(err, git.ErrRepositoryAlreadyExists) {
		return fmt.Errorf("failed to clone repository: %w", err)
	}
	// Get the worktree
	w, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	if g.branch != "" {
		head, err := r.Head()
		if err != nil {
			return fmt.Errorf("failed to get HEAD: %w", err)
		}
		currentBranch := head.Name().String()
		expectedBranchRef := fmt.Sprintf("refs/heads/%s", localBranch)

		// If we're already on the target branch, skip checkout
		if currentBranch != expectedBranchRef {
			// For tracking remote branch, first make sure we have latest remotes
			err = r.Fetch(&git.FetchOptions{
				Auth: g.auth,
			})
			if err != nil && err != git.NoErrAlreadyUpToDate {
				return fmt.Errorf("failed to fetch: %w", err)
			}

			// Check if the local branch already exists
			localBranchRef := plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", localBranch))
			_, err = r.Reference(localBranchRef, true)
			branchExists := err == nil

			// Create checkout options with appropriate Create flag
			checkoutOpts := &git.CheckoutOptions{
				Branch: localBranchRef,
				Create: !branchExists, // Only create if branch doesn't exist
				Force:  true,
			}

			// If we need to create the branch, get the remote reference
			if !branchExists {
				// Get the remote branch reference
				remoteBranchRef := plumbing.ReferenceName(fmt.Sprintf("refs/remotes/%s", remoteBranch))
				remoteRef, err := r.Reference(remoteBranchRef, true)
				if err != nil {
					return fmt.Errorf("failed to find remote branch: %w", err)
				}

				// Set the hash for the new branch
				checkoutOpts.Hash = remoteRef.Hash()
			}

			// Perform the checkout
			err = w.Checkout(checkoutOpts)
			if err != nil {
				return fmt.Errorf("failed to checkout: %w", err)
			}
		} else {
			stale = true
		}
	}

	if stale {
		err = w.PullContext(ctx, &git.PullOptions{
			Auth: g.auth,
		})
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

func GetCurrentBranchFromRepository(repository *git.Repository) (string, error) {
	branchRefs, err := repository.Branches()
	if err != nil {
		return "", err
	}

	headRef, err := repository.Head()
	if err != nil {
		return "", err
	}

	var currentBranchName string
	err = branchRefs.ForEach(func(branchRef *plumbing.Reference) error {
		if branchRef.Hash() == headRef.Hash() {
			currentBranchName = branchRef.Name().String()

			return nil
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	return currentBranchName, nil
}

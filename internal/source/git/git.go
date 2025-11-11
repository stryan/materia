package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	xssh "golang.org/x/crypto/ssh"
)

type GitSource struct {
	activeBranch     string
	defaultBranch    string
	localRepository  string
	remoteRepository string
	auth             transport.AuthMethod
	resetIfNeeded    bool
}

func NewGitSource(c *Config) (*GitSource, error) {
	if c == nil {
		return nil, errors.New("need git config")
	}
	g := &GitSource{
		localRepository: c.LocalRepository,
		resetIfNeeded:   !c.Careful,
		defaultBranch:   "master",
	}
	if c.DefaultBranch != "" {
		g.defaultBranch = c.DefaultBranch
	}
	splitURL := strings.Split(c.URL, "://")
	if splitURL[0] == "git" {
		// We didn't specify the source type and guessed off the URL
		// rewrite to HTTP(S)
		prefix := "https"
		if c.Insecure {
			prefix = "http"
		}
		g.remoteRepository = fmt.Sprintf("%v://%v", prefix, splitURL[1])
	} else {
		// we specified the type directly, use the URL as is
		g.remoteRepository = c.URL
	}

	g.activeBranch = c.Branch
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
		if c.Insecure {
			publicKeys.HostKeyCallback = xssh.InsecureIgnoreHostKey()
		}

		g.auth = publicKeys
	} else if c.Username != "" {
		g.auth = &http.BasicAuth{
			Username: c.Username,
			Password: c.Password,
		}
	}
	return g, nil
}

func (g *GitSource) Sync(ctx context.Context) error {
	localPath := g.localRepository
	localBranch := g.activeBranch
	repoURL := g.remoteRepository
	stale := false
	changedBranch := false
	// Clone the repository
	gco := &git.CloneOptions{
		URL:               repoURL,
		Auth:              g.auth,
		Progress:          os.Stdout,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	}
	r, err := git.PlainCloneContext(ctx, localPath, false, gco)
	if errors.Is(err, git.ErrRepositoryAlreadyExists) {
		r, err = git.PlainOpen(localPath)
		if err != nil {
			log.Fatal(err)
		}
		remote, err := r.Remote("origin")
		if err != nil {
			return fmt.Errorf("failed to get remote: %w", err)
		}
		urls := remote.Config().URLs
		if !slices.Contains[[]string](urls, g.remoteRepository) {
			log.Warn("Repository exists but has different remote URL")
			if g.resetIfNeeded {
				err := g.Clean()
				if err != nil {
					return fmt.Errorf("error cleaning repo: %w", err)
				}
				r, err = git.PlainCloneContext(ctx, localPath, false, gco)
				if err != nil {
					return fmt.Errorf("failed to clone after resetting, giving up: %w", err)
				}
			}
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
	head, err := r.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}
	currentBranch := head.Name().String()
	if g.activeBranch != "" {
		// make sure we're on the specified branch
		expectedBranchRef := fmt.Sprintf("refs/heads/%s", localBranch)

		// If we're already on the target branch, skip checkout
		if currentBranch != expectedBranchRef {
			err = g.checkoutBranch(ctx, r, g.activeBranch)
			if err != nil {
				return fmt.Errorf("error checking out requested branch: %w", err)
			}
			changedBranch = true
		} else {
			stale = true
		}
	} else {
		// make sure we're on the default branch
		defaultBranch, err := g.GetDefaultBranchFromRepository(r)
		if err != nil {
			return fmt.Errorf("error fetching default branch: %w", err)
		}
		currentBranch, err := GetCurrentBranchFromRepository(r)
		if err != nil {
			return fmt.Errorf("error getting current branch: %w", err)
		}
		if currentBranch != defaultBranch {
			err = g.checkoutBranch(ctx, r, defaultBranch)
			if err != nil {
				return fmt.Errorf("error reverting to default branch: %w", err)
			}
			changedBranch = true
		} else {
			stale = true
		}
	}

	if stale || changedBranch {
		err = w.PullContext(ctx, &git.PullOptions{
			Auth:  g.auth,
			Force: g.resetIfNeeded,
		})
		if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
			if errors.Is(err, git.ErrFastForwardMergeNotPossible) {
				if !g.resetIfNeeded {
					return err
				}
				err = g.HardReset(r)
				if err != nil {
					return fmt.Errorf("failed to hard reset when requested: %w", err)
				}
				err = w.PullContext(ctx, &git.PullOptions{
					Auth:  g.auth,
					Force: g.resetIfNeeded,
				})
				if err != nil {
					return fmt.Errorf("failed to pull after hard reseting: %w", err)
				}
			}
		}
	}
	return nil
}

func (g *GitSource) Close(ctx context.Context) error {
	return nil
}

func (g *GitSource) Clean() (_ error) {
	return os.RemoveAll(g.localRepository)
}

func (g *GitSource) HardReset(r *git.Repository) error {
	w, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}
	head, err := r.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD for reset: %w", err)
	}
	return w.Reset(&git.ResetOptions{
		Commit: head.Hash(),
		Mode:   git.HardReset,
	})
}

func (g *GitSource) GetDefaultBranchFromRepository(repo *git.Repository) (string, error) {
	if g.defaultBranch != "" {
		return g.defaultBranch, nil
	}
	remote, err := repo.Remote("origin")
	if err != nil {
		return "", err
	}
	references, _ := remote.List(&git.ListOptions{})
	defaultName := "master"
	for _, reference := range references {
		if reference.Name() == "HEAD" && reference.Type() == plumbing.SymbolicReference {
			defaultName = reference.Target().Short()
			break
		}
	}
	return defaultName, nil
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

func (g *GitSource) fetchOrigin(ctx context.Context, repo *git.Repository, refSpecStr string) error {
	remote, err := repo.Remote("origin")
	if err != nil {
		return err
	}

	var refSpecs []config.RefSpec
	if refSpecStr != "" {
		refSpecs = []config.RefSpec{config.RefSpec(refSpecStr)}
	}

	if err = remote.FetchContext(ctx, &git.FetchOptions{
		RefSpecs: refSpecs,
		Auth:     g.auth,
	}); err != nil {
		if err == git.NoErrAlreadyUpToDate {
			fmt.Print("refs already up to date")
		} else {
			return fmt.Errorf("fetch origin failed: %v", err)
		}
	}

	return nil
}

func (g *GitSource) checkoutBranch(ctx context.Context, r *git.Repository, branch string) error {
	w, err := r.Worktree()
	if err != nil {
		return err
	}

	// ... checking out branch
	branchRefName := plumbing.NewBranchReferenceName(branch)
	branchCoOpts := git.CheckoutOptions{
		Branch: plumbing.ReferenceName(branchRefName),
		Force:  true,
	}
	if err := w.Checkout(&branchCoOpts); err != nil {
		mirrorRemoteBranchRefSpec := fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch)
		err = g.fetchOrigin(ctx, r, mirrorRemoteBranchRefSpec)
		if err != nil {
			return err
		}

		err = w.Checkout(&branchCoOpts)
		if err != nil {
			return err
		}
	}
	return nil
}

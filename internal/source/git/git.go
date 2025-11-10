package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	xssh "golang.org/x/crypto/ssh"
)

type GitSource struct {
	remoteRepository string
	branch           string
	localRepository  string
	auth             transport.AuthMethod
	insecure         bool
	defaultBranch    string
	proto            string
}

func NewGitSource(c *Config) (*GitSource, error) {
	if c == nil {
		return nil, errors.New("need git config")
	}
	g := &GitSource{
		localRepository: c.LocalRepository,
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

	// TODO this is all awful code, redo
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
		g.proto = "ssh"
	} else if c.Username != "" {
		g.auth = &http.BasicAuth{
			Username: c.Username,
			Password: c.Password,
		}
		g.proto = "http"

	}
	return g, nil
}

func (g *GitSource) Sync(ctx context.Context) error {
	localPath := g.localRepository
	localBranch := g.branch
	remoteBranch := fmt.Sprintf("origin/%v", g.branch)
	repoURL := g.remoteRepository
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
	head, err := r.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}
	currentBranch := head.Name().String()
	if g.branch != "" {
		// make sure we're on the specified branch
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
			err = g.CheckoutBranch(r, defaultBranch)
			if err != nil {
				return fmt.Errorf("error reverting to default branch: %w", err)
			}
		}
	}

	if stale {
		err = w.PullContext(ctx, &git.PullOptions{
			Auth: g.auth,
		})
		// TODO if err is ErrFastForwardMergeNotPossible need to reset
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
	return os.RemoveAll(g.localRepository)
}

func (g *GitSource) CheckoutBranch(r *git.Repository, branchName string) error {
	w, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	localBranch := branchName
	remoteBranch := fmt.Sprintf("origin/%v", branchName)
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
	return nil
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

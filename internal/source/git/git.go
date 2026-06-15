package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	xssh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"primamateria.systems/materia/pkg/source"
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
	}
	if c.Default != "" {
		g.defaultBranch = c.Default
	}
	proto := ""
	g.remoteRepository = c.URL
	ep, err := transport.NewEndpoint(c.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse endpoint: %v", err)
	}
	proto = ep.Protocol

	g.activeBranch = c.Branch

	if c.PrivateKey != "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		hostsfile := c.KnownHosts
		if hostsfile == "" {
			hostsfile = fmt.Sprintf("%v/.ssh/known_hosts", home)
			_, err = os.Stat(hostsfile)
			if err != nil {
				return nil, err
			}

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
		if hostsfile != "" {
			publicKeys.HostKeyCallback, err = knownhosts.New(hostsfile)
			if err != nil {
				return nil, fmt.Errorf("can't use knownhosts %v: %w", hostsfile, err)
			}
		}

		g.auth = publicKeys
	} else if c.Username != "" {
		g.auth = &http.BasicAuth{
			Username: c.Username,
			Password: c.Password,
		}
	} else if proto == "ssh" {
		// we want to try to find any private keys in $HOME/.ssh to use here
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		privkey := filepath.Join(home, ".ssh", "id_rsa")
		_, err = os.Stat(privkey)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if !errors.Is(err, os.ErrNotExist) {
			publicKeys, err := ssh.NewPublicKeysFromFile("git", privkey, "")
			if err != nil {
				return nil, err
			}
			if c.Insecure {
				publicKeys.HostKeyCallback = xssh.InsecureIgnoreHostKey()
			}
			g.auth = publicKeys
		}
	}
	return g, nil
}

func (g *GitSource) Sync(ctx context.Context, opts source.SyncOpts) (*source.SyncReport, error) {
	repoURL := g.remoteRepository
	report := &source.SyncReport{}
	gco := &git.CloneOptions{
		URL:               repoURL,
		Auth:              g.auth,
		Progress:          os.Stdout,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	}
	r, oldRevision, err := g.createOrOpenRepo(ctx, gco)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}
	report.OldRevision = oldRevision

	if opts.Revision == "" {
		if err := g.ensureBranch(ctx, r, opts.Subpath); err != nil {
			return nil, err
		}
		target := g.activeBranch
		if target == "" {
			var err error
			target, err = g.GetDefaultBranchFromRepository(r)
			if err != nil {
				return nil, fmt.Errorf("error fetching default branch: %w", err)
			}
		}

		current, err := GetCurrentBranchFromRepository(r)
		if err != nil {
			return nil, fmt.Errorf("error getting current branch: %w", err)
		}

		if current != fmt.Sprintf("refs/heads/%s", target) {
			err = g.checkoutBranch(ctx, r, target, opts.Subpath)
			if err != nil {
				return nil, fmt.Errorf("failed to checkout branch: %w", err)
			}
		}
		if err := g.pull(ctx, r); err != nil {
			return nil, err
		}
	} else {
		if err := g.checkoutRevision(ctx, r, opts.Revision); err != nil {
			return nil, fmt.Errorf("failed to checkout revision %v: %w", opts.Revision, err)
		}
	}
	newHead, err := r.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD after sync: %w", err)
	}
	report.NewRevision = newHead.Hash().String()
	return report, nil
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

// createOrOpenRepo clones a new repo or opens an existing one. Returns the current revision (if existing repo)
func (g *GitSource) createOrOpenRepo(ctx context.Context, opts *git.CloneOptions) (*git.Repository, string, error) {
	r, err := git.PlainCloneContext(ctx, g.localRepository, false, opts)
	if err == nil {
		// fresh start, nothing else to do
		return r, "", nil
	}
	if !errors.Is(err, git.ErrRepositoryAlreadyExists) {
		return nil, "", fmt.Errorf("failed to clone repository: %w", err)
	}

	r, err = git.PlainOpen(g.localRepository)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open repository: %w", err)
	}

	remote, err := r.Remote("origin") // TODO allow custom remotes
	if err != nil {
		return nil, "", fmt.Errorf("failed to get remote: %w", err)
	}
	if slices.Contains(remote.Config().URLs, g.remoteRepository) {
		head, err := r.Head()
		if err != nil {
			return nil, "", fmt.Errorf("failed to get HEAD: %w", err)
		}
		return r, head.Hash().String(), nil
	}

	if !g.resetIfNeeded {
		return nil, "", fmt.Errorf("local repo has different remote and reset is disabled")
	}
	if err := g.Clean(); err != nil {
		return nil, "", fmt.Errorf("error cleaning repo: %w", err)
	}
	r, err = git.PlainCloneContext(ctx, g.localRepository, false, opts)
	if err != nil {
		return nil, "", fmt.Errorf("failed to clone after resetting: %w", err)
	}
	return r, "", nil
}

func (g *GitSource) pull(ctx context.Context, r *git.Repository) error {
	w, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}
	err = w.PullContext(ctx, &git.PullOptions{
		Auth:  g.auth,
		Force: g.resetIfNeeded,
	})
	if err == nil || errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}
	if !errors.Is(err, git.ErrFastForwardMergeNotPossible) || !g.resetIfNeeded {
		return err
	}
	if err := g.HardReset(r); err != nil {
		return fmt.Errorf("failed to hard reset: %w", err)
	}
	if err := w.PullContext(ctx, &git.PullOptions{Auth: g.auth, Force: true}); err != nil {
		return fmt.Errorf("failed to pull after hard reset: %w", err)
	}
	return nil
}

func (g *GitSource) ensureBranch(ctx context.Context, r *git.Repository, subpath string) error {
	target := g.activeBranch
	if target == "" {
		var err error
		target, err = g.GetDefaultBranchFromRepository(r)
		if err != nil {
			return fmt.Errorf("error fetching default branch: %w", err)
		}
	}

	current, err := GetCurrentBranchFromRepository(r)
	if err != nil {
		return fmt.Errorf("error getting current branch: %w", err)
	}

	if current == fmt.Sprintf("refs/heads/%s", target) {
		return nil
	}
	return g.checkoutBranch(ctx, r, target, subpath)
}

func (g *GitSource) checkoutBranch(ctx context.Context, r *git.Repository, branch, subpath string) error {
	w, err := r.Worktree()
	if err != nil {
		return err
	}

	branchRefName := plumbing.NewBranchReferenceName(branch)
	branchCoOpts := git.CheckoutOptions{
		Branch: plumbing.ReferenceName(branchRefName),
		Force:  true,
	}

	if subpath != "" {
		branchCoOpts.SparseCheckoutDirectories = []string{subpath}
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

func (g *GitSource) checkoutRevision(_ context.Context, r *git.Repository, revision string) error {
	w, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	hash, err := r.ResolveRevision(plumbing.Revision(revision))
	if err != nil {
		return fmt.Errorf("failed to resolve revision %q: %w", revision, err)
	}

	return w.Checkout(&git.CheckoutOptions{
		Hash:  *hash,
		Force: g.resetIfNeeded,
	})
}

func (g *GitSource) Inspect() source.SyncInspectReport {
	return source.SyncInspectReport{
		SupportsRollback: true,
	}
}

func (g *GitSource) String() string {
	return fmt.Sprintf("git:%v:%v", g.remoteRepository, g.localRepository)
}

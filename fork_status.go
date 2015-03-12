package main

import (
	"errors"
	"fmt"
	"github.com/google/go-github/github"
	"strings"
	"time"
)

const MAX_BRANCH_COUNT = 25

type ForkStatus struct {
	Owner       string
	Repo        string
	ParentOwner string
	ParentRepo  string
	Branches    map[string]*BranchStatus
}

type BranchStatus struct {
	SHA       string
	ParentSHA string
}

type Fork struct {
	client *github.Client
	owner  string
	repo   string
}

func (f *Fork) getRepo(owner, repo string) *github.Repository {
	r, _, err := f.client.Repositories.Get(owner, repo)

	if err != nil {
		panic(err)
	}

	return r
}

func (f *Fork) getHeads(owner, repo string) []github.Reference {
	options := &github.ReferenceListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
		Type:        "heads",
	}
	refs, _, err := f.client.Git.ListRefs(owner, repo, options)

	if err != nil {
		panic(err)
	}

	return refs
}

func (f *Fork) getStatus() *ForkStatus {
	status := &ForkStatus{
		Owner:    f.owner,
		Repo:     f.repo,
		Branches: make(map[string]*BranchStatus),
	}

	fork := f.getRepo(f.owner, f.repo)
	for _, ref := range f.getHeads(f.owner, f.repo) {
		if *ref.Object.Type != "commit" {
			continue
		}
		name := strings.TrimPrefix(*ref.Ref, "refs/heads/")
		status.Branches[name] = &BranchStatus{
			SHA: *ref.Object.SHA,
		}
	}

	if fork.Parent != nil {
		status.ParentOwner = *fork.Parent.Owner.Login
		status.ParentRepo = *fork.Parent.Name

		for _, ref := range f.getHeads(status.ParentOwner, status.ParentRepo) {
			if *ref.Object.Type != "commit" {
				continue
			}
			name := strings.TrimPrefix(*ref.Ref, "refs/heads/")
			branch, ok := status.Branches[name]
			if !ok {
				branch = &BranchStatus{}
				status.Branches[name] = branch
			}
			branch.ParentSHA = *ref.Object.SHA
		}
	}

	return status
}

func (f *Fork) GetStatus() (status *ForkStatus, err error) {
	defer func() {
		// private methods will panic if there's an error
		if e := recover(); e != nil {
			err = e.(error)
		}
	}()

	return f.getStatus(), nil
}

func (f *Fork) backupBranch(prefix, name, sha string) *github.Reference {
	path := fmt.Sprintf("refs/backups/%s/%s", prefix, name)
	ref := &github.Reference{
		Ref:    &path,
		Object: &github.GitObject{SHA: &sha},
	}

	fmt.Printf("create %s: %s\n", path, sha)
	ref, _, err := f.client.Git.CreateRef(f.owner, f.repo, ref)

	if err != nil {
		panic(err)
	}

	return ref
}

func (f *Fork) backupBranches(status *ForkStatus) []github.Reference {
	prefix := time.Now().Format("20060102220405")

	refs := make([]github.Reference, 0, len(status.Branches))
	for name, branch := range status.Branches {
		refs = append(refs, *f.backupBranch(prefix, name, branch.SHA))
	}

	return refs
}

func (f *Fork) resetBranch(name string, branch *BranchStatus) *github.Reference {
	var (
		ref *github.Reference
		err error
	)
	path := fmt.Sprintf("refs/heads/%s", name)

	switch {
	case branch.SHA == branch.ParentSHA:
		fmt.Printf("noop %s\n", name)

	case branch.SHA == "" && branch.ParentSHA != "":
		fmt.Printf("create %s: => %s\n", name, branch.ParentSHA)
		ref, _, err = f.client.Git.CreateRef(f.owner, f.repo, &github.Reference{
			Ref:    &path,
			Object: &github.GitObject{SHA: &branch.ParentSHA},
		})

	case branch.SHA != "" && branch.ParentSHA != "":
		fmt.Printf("update %s: %s => %s\n", name, branch.SHA, branch.ParentSHA)
		ref, _, err = f.client.Git.UpdateRef(f.owner, f.repo, &github.Reference{
			Ref:    &path,
			Object: &github.GitObject{SHA: &branch.ParentSHA},
		}, true)

	case branch.SHA == "" && branch.ParentSHA != "":
		fmt.Printf("delete %s: %s => \n", name, branch.SHA)
		_, err = f.client.Git.DeleteRef(f.owner, f.repo, path)

	default:
		err = errors.New("SHA and ParentSHA can not be empty")
	}

	if err != nil {
		panic(err)
	}

	return ref
}

func (f *Fork) resetBranches(status *ForkStatus) []*github.Reference {
	refs := make([]*github.Reference, 0, len(status.Branches))
	for name, branch := range status.Branches {
		refs = append(refs, f.resetBranch(name, branch))
	}

	return refs
}

func (f *Fork) Reset() (err error) {
	defer func() {
		// private methods will panic if there's an error
		if e := recover(); e != nil {
			err = e.(error)
		}
	}()

	initialStatus := f.getStatus()

	if len(initialStatus.Branches) > MAX_BRANCH_COUNT {
		return errors.New(fmt.Sprintf("Too many branches found. Max %d for "+
			"the unique sum of the repo and parent branches", MAX_BRANCH_COUNT))
	}

	f.backupBranches(initialStatus)
	f.resetBranches(initialStatus)
	return
}

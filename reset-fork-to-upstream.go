package main

import (
	"errors"
	"fmt"
	"github.com/codegangsta/negroni"
	noauth2 "github.com/goincremental/negroni-oauth2"
	sessions "github.com/goincremental/negroni-sessions"
	"github.com/goincremental/negroni-sessions/cookiestore"
	"github.com/google/go-github/github"
	"github.com/gorilla/mux"
	"golang.org/x/oauth2"
	"net/http"
	"os"
	"strings"
	"time"
)

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
		panic(err.Error())
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
		panic(err.Error())
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
	f.backupBranches(initialStatus)
	f.resetBranches(initialStatus)
	return
}

func main() {

	oauthProvider := noauth2.Github(&noauth2.Config{
		ClientID:     os.Getenv("CLIENT_ID"),
		ClientSecret: os.Getenv("CLIENT_SECRET"),
		RedirectURL:  os.Getenv("REDIRECT_URL"),
		Scopes:       []string{"repo"},
	})

	secureRouter := mux.NewRouter().PathPrefix("/restrict").Subrouter()

	secureRouter.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		token := noauth2.GetToken(req)
		fmt.Fprintf(w, "OK: %s", token.Access())
	})

	githubClient := func(req *http.Request) *github.Client {
		token := oauth2.Token(noauth2.GetToken(req).Get())
		transport := oauthProvider.Config().Client(oauth2.NoContext, &token)
		return github.NewClient(transport)
	}

	secureRouter.HandleFunc("/repos", func(w http.ResponseWriter, req *http.Request) {
		client := githubClient(req)
		options := &github.RepositoryListOptions{
			ListOptions: github.ListOptions{
				PerPage: 100},
			Type: "heads",
		}
		repos, _, err := client.Repositories.List("", options)

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Problems: %s", err)
			return
		}

		fmt.Fprintf(w, "<table>")
		fmt.Fprintf(w, "<tr><th>%s</th><th>%s</th><th>%s</th></tr>", "Owner", "Repo", "Reset")
		for _, repo := range repos {
			owner := *repo.Owner.Login
			repo := *repo.Name
			url, _ := secureRouter.Get("repo").URL("owner", owner, "repo", repo)
			fmt.Fprintf(w, "<tr><td>%s</td><td><a href=\"%s\">%s</a></td>", owner, url, repo)
			url, _ = secureRouter.Get("repo-reset-post").URL("owner", owner, "repo", repo)
			fmt.Fprintf(w, "<td><form method=\"POST\" action=\"%s\"><input type=\"submit\"/></form></td>", url)
			fmt.Fprintf(w, "</tr>")
		}
		fmt.Fprintf(w, "</table>")
	}).
		Name("repos")

	secureRouter.HandleFunc("/repos/{owner}/{repo}", func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)

		fork := Fork{
			client: githubClient(req),
			owner:  vars["owner"],
			repo:   vars["repo"],
		}

		status, err := fork.GetStatus()

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Problems: %s", err)
			return
		}

		fmt.Fprintf(w, "<h1>%s/%s</h1>", status.Owner, status.Repo)
		fmt.Fprintf(w, "<h2>%v/%v</h2>", status.ParentOwner, status.ParentRepo)

		fmt.Fprintf(w, "<table>")
		fmt.Fprintf(w, "<tr><th>%s</th><th>%s</th><th>%s</th></tr>", "name", "SHA", "ParentSHA")
		for name, shas := range status.Branches {
			fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%s</td></td>", name, shas.SHA, shas.ParentSHA)
		}
		fmt.Fprintf(w, "</table>")
	}).
		Name("repo")

	secureRouter.HandleFunc("/repos/{owner}/{repo}/resets", func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)

		fork := Fork{
			client: githubClient(req),
			owner:  vars["owner"],
			repo:   vars["repo"],
		}

		err := fork.Reset()

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Problems: %s", err)
			return
		}

		fmt.Println("Done")

		url, _ := secureRouter.Get("repo").URL("owner", fork.owner, "repo", fork.repo)
		http.Redirect(w, req, url.String(), http.StatusTemporaryRedirect)

	}).
		Name("repo-reset-post").
		Methods("POST")

	secure := negroni.New()
	secure.Use(noauth2.LoginRequired())
	secure.UseHandler(secureRouter)

	n := negroni.New()
	n.Use(sessions.Sessions("rftu", cookiestore.New([]byte(os.Getenv("COOKIE_SECRET")))))
	n.Use(oauthProvider)

	router := mux.NewRouter()

	router.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		token := noauth2.GetToken(req)
		if token == nil || !token.Valid() {
			fmt.Fprintf(w, "not logged in, or the access token is expired")
			return
		}
		fmt.Fprintf(w, "logged in")
		return
	})

	router.PathPrefix("/restrict").Handler(secure)

	n.UseHandler(router)

	n.Run(":" + os.Getenv("PORT"))
}

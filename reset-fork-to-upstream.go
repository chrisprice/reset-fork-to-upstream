package main

import (
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
	refsOptions := &github.ReferenceListOptions{Type: "heads"}
	refs, _, err := f.client.Git.ListRefs(owner, repo, refsOptions)

	if err != nil {
		panic(err.Error())
	}

	return refs
}

func (f *Fork) GetStatus() (status *ForkStatus, err error) {
	defer func() {
		// private methods will panic if there's an error
		if err := recover(); err != nil {
			err = nil
		}
	}()

	status = &ForkStatus{
		Owner:    f.owner,
		Repo:     f.repo,
		Branches: make(map[string]*BranchStatus),
	}

	fork := f.getRepo(f.owner, f.repo)
	for _, ref := range f.getHeads(f.owner, f.repo) {
		if *ref.Object.Type == "commit" {
			status.Branches[*ref.Ref] = &BranchStatus{
				SHA: *ref.Object.SHA,
			}
		}
	}

	if fork.Parent != nil {
		status.ParentOwner = *fork.Parent.Owner.Login
		status.ParentRepo = *fork.Parent.Name

		for _, ref := range f.getHeads(status.ParentOwner, status.ParentRepo) {
			if *ref.Object.Type != "commit" {
				continue
			}
			branch, ok := status.Branches[*ref.Ref]
			if !ok {
				branch = &BranchStatus{}
				status.Branches[*ref.Ref] = branch
			}
			branch.ParentSHA = *ref.Object.SHA
		}
	}

	return status, nil
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
		repos, _, err := client.Repositories.List("", nil)

		if err != nil {
			fmt.Fprintf(w, "Problems: %s", err)
			return
		}

		fmt.Fprintf(w, "<table>")
		for _, repo := range repos {
			owner := *repo.Owner.Login
			repo := *repo.Name
			url, _ := secureRouter.Get("repo").URL("owner", owner, "repo", repo)
			fmt.Fprintf(w, "<tr><td>%s</td><td><a href=\"%s\">%s</a></td></tr>", owner, url, repo)
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
			fmt.Fprintf(w, "Problems: %s", err)
			return
		}

		fmt.Fprintf(w, "<h1>%s/%s</h1>", status.Owner, status.Repo)
		fmt.Fprintf(w, "<h2>%v/%v</h2>", status.ParentOwner, status.ParentRepo)

		fmt.Fprintf(w, "<table>")
		fmt.Fprintf(w, "<tr><th>%s</th><th>%s</th><th>%s</th></tr>", "name", "SHA", "ParentSHA")
		for name, shas := range status.Branches {
			fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%s</td></tr>", name, shas.SHA, shas.ParentSHA)
		}
		fmt.Fprintf(w, "</table>")
	}).
		Name("repo")

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

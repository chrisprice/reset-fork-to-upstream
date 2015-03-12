package main

import (
	"github.com/codegangsta/negroni"
	sessions "github.com/goincremental/negroni-sessions"
	cookies "github.com/goincremental/negroni-sessions/cookiestore"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/unrolled/render"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"net/http"
	"os"
)

const securePrefix string = "/repos"

func configureSecureRoutes(secureRouter *mux.Router, oauth *OAuth, render *render.Render) {
	secureRouter.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		client := oauth.GetGithubClient(req)

		repos, err := ListRepos(client)

		if err != nil {
			render.JSON(w, http.StatusInternalServerError, err)
			return
		}

		render.JSON(w, http.StatusOK, repos)
	}).
		Name("repos")

	secureRouter.HandleFunc("/{owner}/{repo}", func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)

		fork := Fork{
			client: oauth.GetGithubClient(req),
			owner:  vars["owner"],
			repo:   vars["repo"],
		}

		status, err := fork.GetStatus()

		if err != nil {
			render.JSON(w, http.StatusInternalServerError, err)
			return
		}

		render.JSON(w, http.StatusOK, status)
	}).
		Name("repo")

	secureRouter.HandleFunc("/{owner}/{repo}/resets", func(w http.ResponseWriter, req *http.Request) {
		// Probably a better way to do this
		chocolateDigestive, err := req.Cookie("session")
		if err != nil || req.Header.Get("X-Csrf-Token") != chocolateDigestive.Value {
			render.JSON(w, http.StatusUnauthorized, "CSRF failure")
			return
		}

		vars := mux.Vars(req)

		fork := Fork{
			client: oauth.GetGithubClient(req),
			owner:  vars["owner"],
			repo:   vars["repo"],
		}

		if err := fork.Reset(); err != nil {
			render.JSON(w, http.StatusInternalServerError, err)
			return
		}

		url, _ := secureRouter.Get("repo").URL("owner", fork.owner, "repo", fork.repo)
		http.Redirect(w, req, url.String(), http.StatusTemporaryRedirect)

	}).
		Name("repo-reset-post").
		Methods("POST")
}

func main() {
	appURL := os.Getenv("APP_URL")
	oauth := &OAuth{oauth2.Config{
		ClientID:     os.Getenv("CLIENT_ID"),
		ClientSecret: os.Getenv("CLIENT_SECRET"),
		RedirectURL:  os.Getenv("REDIRECT_URL"),
		Scopes:       []string{"repo"},
		Endpoint:     github.Endpoint,
	}}
	render := render.New()

	secureRouter := mux.NewRouter().PathPrefix(securePrefix).Subrouter()
	configureSecureRoutes(secureRouter, oauth, render)

	secure := negroni.New()
	secure.Use(GetLoginRequired())
	secure.UseHandler(secureRouter)

	unsecureRouter := mux.NewRouter()
	unsecureRouter.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, appURL, http.StatusMovedPermanently)
	})
	unsecureRouter.PathPrefix(securePrefix).Handler(secure)

	unsecure := negroni.New()
	unsecure.Use(cors.New(cors.Options{
		AllowedOrigins:   []string{"http://locahost", appURL},
		AllowedMethods:   []string{"GET", "POST"},
		AllowCredentials: true}))
	unsecure.Use(sessions.Sessions("session", cookies.New([]byte(os.Getenv("COOKIE_SECRET")))))
	unsecure.Use(oauth.GetOAuth2Provider())
	unsecure.UseHandler(unsecureRouter)

	unsecure.Run(":" + os.Getenv("PORT"))
}

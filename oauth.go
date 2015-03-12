package main

import (
	"github.com/codegangsta/negroni"
	noauth2 "github.com/goincremental/negroni-oauth2"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"net/http"
)

type OAuth struct {
	oauth2.Config
}

func (config *OAuth) GetGithubClient(req *http.Request) *github.Client {
	nToken := noauth2.GetToken(req).Get()
	oToken := oauth2.Token(nToken)
	transport := config.Client(oauth2.NoContext, &oToken)
	return github.NewClient(transport)
}

func (config *OAuth) GetOAuth2Provider() negroni.HandlerFunc {
	return noauth2.NewOAuth2Provider(&noauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Scopes:       config.Scopes,
		RedirectURL:  config.RedirectURL,
	}, config.Endpoint.AuthURL, config.Endpoint.TokenURL)
}

// Alias to simplify the imports in server.go
func GetLoginRequired() negroni.Handler {
	return noauth2.LoginRequired()
}

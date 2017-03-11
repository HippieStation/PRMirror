package main

import (
	"context"
	"os"

	"github.com/google/go-github/github"
	"github.com/op/go-logging"
	"golang.org/x/oauth2"
)

var PRMirrorer = PRMirror{}

func main() {
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, format)
	logging.SetBackend(backendFormatter)

	Configuration := Config{}.Init()

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: Configuration.GitHubToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	PRMirrorer = PRMirror{
		GitHubClient:  client,
		Context:       &ctx,
		Configuration: &Configuration,
	}

	PRMirrorer.Run()
}

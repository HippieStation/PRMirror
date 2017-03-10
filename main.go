package main

import (
	"context"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

var PRMirrorer = PRMirror{}

func main() {
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

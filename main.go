package main

import (
	"context"
	"os"

	"flag"

	"github.com/google/go-github/github"
	"github.com/op/go-logging"
	"golang.org/x/oauth2"
)

var PRMirrorer = PRMirror{}
var InitialImport = false

func init() {
	flag.BoolVar(&InitialImport, "InitialImport", false, "Import all current PRs from Upstream")
	flag.Parse()
}

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

	if InitialImport {
		PRMirrorer.InitialImport()
	} else {
		PRMirrorer.Run()
	}
}

package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/google/go-github/github"
	"github.com/op/go-logging"
	"golang.org/x/oauth2"
)

var Configuration = Config{}

var log = logging.MustGetLogger("PRMirror")

var format = logging.MustStringFormatter(
	`%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}`,
)

func main() {

	backend := logging.NewLogBackend(os.Stderr, "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, format)
	logging.SetBackend(backendFormatter)

	Configuration = Configuration.Init()

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: Configuration.GitHubToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)
	events, _, err := client.Activity.ListRepositoryEvents(ctx, Configuration.UpstreamOwner, Configuration.UpstreamRepo, nil)
	if _, ok := err.(*github.RateLimitError); ok {
		// TODO: Maybe add some context here
		log.Error("The github.com rate limit has been hit")
	}

	for _, event := range events {

		if *event.Type == "PullRequestEvent" {
			prEvent := github.PullRequestEvent{}

			err = json.Unmarshal(event.GetRawPayload(), &prEvent)
			if err != nil {
				panic(err)
			}

			prAction := prEvent.GetAction()

			log.Debugf("%s", prEvent.PullRequest.GetURL())

			if prAction == "opened" {
				//TODO: Check if we already have an open PR for this and add a comment saying upstream reopened it and remove the upsteam closed tag
				MirrorPR(&prEvent)
			} else if prAction == "closed" {
				//AddLabel("Upstream Closed")
			}
		}
	}
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/go-github/github"
)

type MirroredPR struct {
	DownstreamID int
	UpstreamID   int
}

type PRMirror struct {
	GitHubClient  *github.Client
	Context       *context.Context
	Configuration *Config
}

func (p PRMirror) Run() {
	events, _, err := p.GitHubClient.Activity.ListRepositoryEvents(*p.Context, p.Configuration.UpstreamOwner, p.Configuration.UpstreamRepo, nil)
	if _, ok := err.(*github.RateLimitError); ok {
		log.Println("hit rate limit")
	}

	for _, event := range events {

		if *event.Type == "PullRequestEvent" {
			prEvent := github.PullRequestEvent{}

			err = json.Unmarshal(event.GetRawPayload(), &prEvent)
			if err != nil {
				panic(err)
			}

			prAction := prEvent.GetAction()

			fmt.Printf("%s\n", prEvent.PullRequest.GetURL())

			if prAction == "opened" {
				//TODO: Check if we already have an open PR for this and add a comment saying upstream reopened it and remove the upsteam closed tag
				p.MirrorPR(&prEvent)
			} else if prAction == "closed" {
				//AddLabel("Upstream Closed")
			}
		}
	}
}

func (p PRMirror) MirrorPR(PREvent *github.PullRequestEvent) {
	fmt.Printf("Mirroring new PR: %s\n", PREvent.PullRequest.GetTitle())
}

func (p PRMirror) AddLabel(id int, tag string) {
	//client.Issues.AddLabelsToIssue
}

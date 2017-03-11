package main

import (
	"context"
	"encoding/json"

	"time"

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
	Database      *Database
}

func (p PRMirror) HandlePREvent(prEvent *github.PullRequestEvent) {
	prAction := prEvent.GetAction()

	log.Debugf("%s\n", prEvent.PullRequest.GetURL())

	if prAction == "opened" {
		//TODO: Check if we already have an open PR for this and add a comment saying upstream reopened it and remove the upsteam closed tag
		p.MirrorPR(prEvent.PullRequest)
	} else if prAction == "closed" {

		//AddLabel("Upstream Closed")
	}
}

func (p PRMirror) isRatelimit(err error) bool {
	if _, ok := err.(*github.RateLimitError); ok {
		// TODO: Maybe add some context here
		log.Error("The github.com rate limit has been hit")
		return true
	}
	return false
}

func (p PRMirror) GetOpenPRs() ([]*github.PullRequest, error) {
	var allPrs []*github.PullRequest

	opt := &github.PullRequestListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		log.Debugf("Getting OpenPRs Page %d\n", opt.ListOptions.Page)

		prs, resp, err := p.GitHubClient.PullRequests.List(*p.Context, p.Configuration.UpstreamOwner, p.Configuration.UpstreamRepo, opt)
		if p.isRatelimit(err) {
			return nil, err
		}

		allPrs = append(allPrs, prs...)
		if resp.NextPage == 0 {
			break
		}
		opt.ListOptions.Page = resp.NextPage
	}

	return allPrs, nil
}

func (p PRMirror) InitialImport() {
	prs, err := p.GetOpenPRs()
	if p.isRatelimit(err) {
		return
	}

	for _, pr := range prs {
		prNum, err := p.Database.GetDownstreamID(pr.GetNumber())
		if err != nil {
			panic(err)
		}

		if prNum != 0 {
			log.Infof("DUP: [%d] - %s\n", pr.GetNumber(), pr.GetTitle())
		} else {
			log.Infof("NEW: [%d] - %s\n", pr.GetNumber(), pr.GetTitle())
			prID, err := p.MirrorPR(pr)
			if err != nil {
				panic(err)
			}
			p.Database.StoreMirror(prID, pr.GetNumber())

			time.Sleep(2 * time.Second)
		}
	}
}

func (p PRMirror) Run() {
	events, _, err := p.GitHubClient.Activity.ListRepositoryEvents(*p.Context, p.Configuration.UpstreamOwner, p.Configuration.UpstreamRepo, nil)
	if p.isRatelimit(err) {
		return
	}

	for _, event := range events {
		eventType := event.GetType()

		if eventType == "PullRequestEvent" {
			prEvent := github.PullRequestEvent{}
			err = json.Unmarshal(event.GetRawPayload(), &prEvent)
			if err != nil {
				panic(err)
			}

			p.HandlePREvent(&prEvent)
		}
	}
}

func (p PRMirror) MirrorPR(pr *github.PullRequest) (int, error) {
	log.Infof("Mirroring PR [%d]: %s from ", pr.GetNumber(), pr.GetTitle(), pr.User.GetLogin())

	base := "master"
	maintainerCanModify := false
	title := "[MIRROR] " + pr.GetTitle()

	newPR := github.NewPullRequest{}
	newPR.Title = &title
	newPR.Body = pr.Body
	newPR.Base = &base
	newPR.Head = pr.Head.Label
	newPR.MaintainerCanModify = &maintainerCanModify

	pr, _, err := p.GitHubClient.PullRequests.Create(*p.Context, p.Configuration.DownstreamOwner, p.Configuration.DownstreamRepo, &newPR)
	if err != nil {
		return 0, err
	}

	return pr.GetNumber(), nil
}

func (p PRMirror) AddLabels(id int, tags []string) bool {
	_, _, err := p.GitHubClient.Issues.AddLabelsToIssue(*p.Context, p.Configuration.DownstreamOwner, p.Configuration.DownstreamOwner, id, tags)
	if err != nil {
		log.Errorf("Error while adding a label to issue#:%d - %s", id, err.Error())
		return false
	}

	return true
}

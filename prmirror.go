package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"os"

	"github.com/google/go-github/github"
)

// MirroredPR contains the upstream and downstream PR ids
type MirroredPR struct {
	DownstreamID int
	UpstreamID   int
}

// PRMirror contains various different variables
type PRMirror struct {
	GitHubClient  *github.Client
	Context       *context.Context
	Configuration *Config
	Database      *Database
}

// GitHubEventMonitor passes in an instance of the PRMirror struct to all HTTP calls to the webhook listener
type GitHubEventMonitor struct {
	Mirrorer PRMirror
}

// HandleEvent handles github events and acts like an event handler
func (p PRMirror) HandleEvent(event *github.Event) {
	seenEvent, _ := p.Database.SeenEvent(event.GetID())
	if seenEvent {
		return
	}

	eventType := event.GetType()
	if eventType != "PullRequestEvent" {
		return
	}

	prEvent := github.PullRequestEvent{}
	err := json.Unmarshal(event.GetRawPayload(), &prEvent)
	if err != nil {
		panic(err)
	}

	log.Debugf("Handling PR Event: %s\n", prEvent.PullRequest.GetURL())

	prAction := prEvent.GetAction()
	if prAction == "closed" && prEvent.PullRequest.GetMerged() == true {
		prID, err := p.MirrorPR(prEvent.PullRequest)
		if err != nil {
			log.Errorf("Error while creating a new PR: %s\n", err.Error())
		} else {
			p.AddLabels(prID, []string{"Upstream PR Merged"})
			p.Database.StoreMirror(prID, prEvent.PullRequest.GetNumber())
		}
	}
	p.Database.AddEvent(event.GetID())
}

// Check if an error is a rate limit error
func (p PRMirror) isRatelimit(err error) bool {
	if _, ok := err.(*github.RateLimitError); ok {
		// TODO: Maybe add some context here
		log.Error("The github.com rate limit has been hit")
		return true
	}
	return false
}

// GetRepoEvents returns a list a list of RepoEvents
func (p PRMirror) GetRepoEvents() ([]*github.Event, int64, error) {
	var allEvents []*github.Event
	var pollInterval = int64(0)

	opt := &github.ListOptions{
		PerPage: 100,
	}

	for {
		log.Debugf("Getting RepoEvents Page %d\n", opt.Page)

		events, resp, err := p.GitHubClient.Activity.ListRepositoryEvents(*p.Context, p.Configuration.UpstreamOwner, p.Configuration.UpstreamRepo, opt)
		if err != nil {
			log.Errorf("Error while listing repository events. %s", err.Error())
			return nil, 60, err
		}

		allEvents = append(allEvents, events...)
		if resp.NextPage == 0 {
			pollInterval, err = strconv.ParseInt(resp.Response.Header.Get("X-Poll-Interval"), 10, 64)
			if err != nil {
				panic(err)
			}
			break
		}
		opt.Page = resp.NextPage
	}

	return allEvents, pollInterval, nil
}

// RunEventScraper runs the GitHub repo event API scraper
func (p PRMirror) RunEventScraper() {
	for {
		events, pollInterval, err := p.GetRepoEvents()
		if err == nil {
			for _, event := range events {
				p.HandleEvent(event)
			}
		}

		log.Debugf("Sleeping for %d as specified by GitHub\n", pollInterval)
		time.Sleep(time.Duration(pollInterval) * time.Second)
	}
}

// ServeHTTP handles HTTP requests to the webhook endpoint
func (s GitHubEventMonitor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, []byte(s.Mirrorer.Configuration.WebhookSecret))
	if err != nil {
		log.Errorf("Error validating the payload\n")
		return
	}
	eventI, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		log.Errorf("Error parsing the payload\n")
	}

	event, ok := eventI.(github.Event)
	if !ok {
		log.Errorf("Received an event which was not an event?: %s\n", eventI)
		return
	}

	s.Mirrorer.HandleEvent(&event)
}

// RunWebhookListener acts a webhook listener which GitHub will call with events
func (p PRMirror) RunWebhookListener() {
	server := GitHubEventMonitor{Mirrorer: p}
	err := http.ListenAndServe(fmt.Sprintf(":%d", p.Configuration.WebhookPort), server)
	log.Fatal(err)
}

// MirrorPR will mirror a PR from an upstream to the downstream
func (p PRMirror) MirrorPR(pr *github.PullRequest) (int, error) {
	log.Infof("Mirroring PR [%d]: %s from %s\n", pr.GetNumber(), pr.GetTitle(), pr.User.GetLogin())

	cmd := exec.Command(fmt.Sprintf("%s%s", p.Configuration.RepoPath, p.Configuration.ToolPath), strconv.Itoa(pr.GetNumber()), pr.GetTitle())
	cmd.Dir = p.Configuration.RepoPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		panic(err)
	}
	err = cmd.Wait()
	if err != nil {
		panic(err)
	}

	cmdoutput, err := cmd.Output()

	base := "master"
	head := fmt.Sprintf("upstream-merge-%d", pr.GetNumber())
	maintainerCanModify := true // We are the owner of the PR so we can specify this as true
	title := fmt.Sprintf("[MIRROR] %s", pr.GetTitle())
	body := fmt.Sprintf("Original PR: %s\n--------------------\n%s", pr.GetHTMLURL(), strings.Replace(pr.GetBody(), "@", "@ ", -1))

	newPR := github.NewPullRequest{}
	newPR.Title = &title
	newPR.Body = &body
	newPR.Base = &base
	newPR.Head = &head
	newPR.MaintainerCanModify = &maintainerCanModify

	pr, _, err = p.GitHubClient.PullRequests.Create(*p.Context, p.Configuration.DownstreamOwner, p.Configuration.DownstreamRepo, &newPR)
	if err != nil {
		return 0, err
	}

	if strings.Contains(string(cmdoutput), "Rejected hunk") {
		p.AddLabels(pr.GetNumber(), []string{"Auto Merge Rejections"})
	}

	return pr.GetNumber(), nil
}

// CreateLabel creates a new label
func (p PRMirror) CreateLabel(labelText string, labelColour string) bool {
	label := github.Label{
		Name:  &labelText,
		Color: &labelColour,
	}

	_, _, err := p.GitHubClient.Issues.CreateLabel(*p.Context, p.Configuration.DownstreamOwner, p.Configuration.DownstreamRepo, &label)
	if err != nil {
		log.Errorf("Error while creating a label - %s", err.Error())
		return false
	}

	return true
}

// AddLabels adds label/s to a pull request
func (p PRMirror) AddLabels(id int, labels []string) bool {
	_, _, err := p.GitHubClient.Issues.AddLabelsToIssue(*p.Context, p.Configuration.DownstreamOwner, p.Configuration.DownstreamRepo, id, labels)
	if err != nil {
		log.Errorf("Error while adding a label on issue#:%d - %s", id, err.Error())
		return false
	}

	return true
}

// RemoveLabel Removes a label from a pull request
func (p PRMirror) RemoveLabel(id int, labels string) bool {
	_, err := p.GitHubClient.Issues.RemoveLabelForIssue(*p.Context, p.Configuration.DownstreamOwner, p.Configuration.DownstreamRepo, id, labels)
	if err != nil {
		log.Errorf("Error while removing a label on issue#:%d - %s", id, err.Error())
		return false
	}

	return true
}

// AddComment adds a comment to a pull request
func (p PRMirror) AddComment(id int, comment string) bool {
	issueComment := github.IssueComment{}
	issueComment.Body = &comment

	_, _, err := p.GitHubClient.Issues.CreateComment(*p.Context, p.Configuration.DownstreamOwner, p.Configuration.DownstreamRepo, id, &issueComment)
	if err != nil {
		log.Errorf("Error while adding a comment to issue#:%d - %s", id, err.Error())
		return false
	}

	return true
}

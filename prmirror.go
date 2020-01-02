package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

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
	GitLock       *SpinLock
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
	if eventType == "PullRequestEvent" && event.GetRepo().GetName() == p.Configuration.UpstreamOwner+"/"+p.Configuration.UpstreamRepo {
		prEvent := github.PullRequestEvent{}
		err := json.Unmarshal(event.GetRawPayload(), &prEvent)
		if err != nil {
			panic(err)
		}

		p.HandlePREvent(&prEvent)
		p.Database.AddEvent(event.GetID())
	} else if eventType == "IssueCommentEvent" && event.GetRepo().GetName() == p.Configuration.DownstreamOwner+"/"+p.Configuration.DownstreamRepo {
		prComment := github.IssueCommentEvent{}
		err := json.Unmarshal(event.GetRawPayload(), &prComment)
		if err != nil {
			panic(err)
		}

		if !prComment.GetIssue().IsPullRequest() {
			return
		}

		p.HandlePRComment(&prComment)
		p.Database.AddEvent(event.GetID())
	}
}

// HandlePREvent handles PR events
func (p PRMirror) HandlePREvent(prEvent *github.PullRequestEvent) {
	//repoName := prEvent.Repo.GetName()
	//repoOwner := prEvent.Repo.Owner.GetName()
	prEventURL := prEvent.PullRequest.GetURL()

	//if p.Configuration.UseWebhook repoName != p.Configuration.DownstreamRepo {
	//	log.Warningf("Ignoring PR Event: %s because %s != %s\n", prEventURL, repoName, p.Configuration.UpstreamRepo)
	//	return
	//} //else if repoOwner != p.Configuration.DownstreamOwner {
	//log.Warningf("Ignoring PR Event: %s because %s != %s\n", prEventURL, repoOwner, p.Configuration.UpstreamOwner)
	//return
	//}

	log.Debugf("Handling PR Event: %s\n", prEventURL)

	prAction := prEvent.GetAction()
	if prAction == "closed" && prEvent.PullRequest.GetMerged() == true && prEvent.PullRequest.Base.GetRef() == "master" {
		prID, err := p.MirrorPR(prEvent.PullRequest)
		if err != nil {
			log.Errorf("Error while creating a new PR: %s\n", err.Error())
		} else {
			p.Database.StoreMirror(prID, prEvent.PullRequest.GetNumber())
		}
	}
}

// HandlePRComment handles comment events
func (p PRMirror) HandlePRComment(prComment *github.IssueCommentEvent) {
	prCommentURL := prComment.GetIssue().GetURL()

	log.Debugf("Handling PR Comment: %s\n", prCommentURL)

	comment := prComment.GetComment()
	rank := comment.GetAuthorAssociation()
	if (rank == "COLLABORATOR" || rank == "MEMBER" || rank == "OWNER") && strings.HasPrefix(comment.GetBody(), "remirror") {
		id := prComment.GetIssue().GetNumber()
		pr, _, err := p.GitHubClient.PullRequests.Get(*p.Context, p.Configuration.DownstreamOwner, p.Configuration.DownstreamRepo, id)
		if err != nil {
			log.Errorf("Error while getting downstream PR for remirror: %s\n", err.Error())
			return
		}
		body := pr.GetBody()
		temp := strings.Split(body, "/")
		temp2 := strings.Split(temp[6], "\n")
		id, err = strconv.Atoi(temp2[0])

		pr, _, err = p.GitHubClient.PullRequests.Get(*p.Context, p.Configuration.UpstreamOwner, p.Configuration.UpstreamRepo, id)
		if err != nil {
			log.Errorf("Error while getting upstream PR to remirror: %s\n", err.Error())
			return
		}

		_, err = p.RemirrorPR(pr)
		if err != nil {
			log.Errorf("Error while remirroring PR: %s\n", err.Error())
		}
	}
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

		// Handle downstream events
		events, pollInterval, err = p.GetDownstreamRepoEvents()
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
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		log.Errorf("Error parsing the payload\n")
	}

	switch event := event.(type) {
	case *github.PullRequestEvent:
		s.Mirrorer.HandlePREvent(event)
	}
}

// RunWebhookListener acts a webhook listener which GitHub will call with events
func (p PRMirror) RunWebhookListener() {
	server := GitHubEventMonitor{Mirrorer: p}
	err := http.ListenAndServe(fmt.Sprintf(":%d", p.Configuration.WebhookPort), server)
	log.Fatal(err)
}

// MirrorPR will mirror a PR from an upstream to the downstream
func (p PRMirror) MirrorPR(pr *github.PullRequest) (int, error) {
	p.GitLock.Lock()
	defer p.GitLock.Unlock()

	downstreamID, err := p.Database.GetDownstreamID(pr.GetNumber())
	if downstreamID != 0 {
		log.Warningf("Refusing to mirror already existing PR: %s - %s\n", pr.GetTitle(), pr.GetNumber())
		return 0, errors.New("prmirror: tried to mirror a PR which has already been mirrored")
	}

	log.Infof("Mirroring PR [%d]: %s from %s\n", pr.GetNumber(), pr.GetTitle(), pr.User.GetLogin())

	cmd := exec.Command(fmt.Sprintf("%s%s", p.Configuration.RepoPath, p.Configuration.ToolPath), strconv.Itoa(pr.GetNumber()), pr.GetTitle())
	cmd.Dir = p.Configuration.RepoPath
	cmdoutput, err := cmd.CombinedOutput()
	if err != nil {
		log.Criticalf("Error while mirroring %d: %s\n", pr.GetNumber(), err)
		return 0, err
	}

	logpath := fmt.Sprintf("./logs/upstream-merge-%d.log", pr.GetNumber())
	ioutil.WriteFile(logpath, cmdoutput, 0600)
	log.Debugf("Wrote log to %s\n", logpath)

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

// RemirrorPR will update the downstream mirror branch
func (p PRMirror) RemirrorPR(pr *github.PullRequest) (int, error) {
	p.GitLock.Lock()
	defer p.GitLock.Unlock()

	//downstreamID, err := p.Database.GetDownstreamID(pr.GetNumber())
	/*if downstreamID != 0 {
		log.Warningf("Refusing to mirror already existing PR: %s - %s\n", pr.GetTitle(), pr.GetNumber())
		return 0, errors.New("prmirror: tried to mirror a PR which has already been mirrored")
	}*/

	log.Infof("Remirroring PR [%d]: %s from %s\n", pr.GetNumber(), pr.GetTitle(), pr.User.GetLogin())

	cmd := exec.Command(fmt.Sprintf("%s%s", p.Configuration.RepoPath, p.Configuration.ToolPath), strconv.Itoa(pr.GetNumber()), pr.GetTitle())
	cmd.Dir = p.Configuration.RepoPath
	cmdoutput, err := cmd.CombinedOutput()
	if err != nil {
		log.Criticalf("Error while remirroring %d: %s\n", pr.GetNumber(), err)
		return 0, err
	}

	logpath := fmt.Sprintf("./logs/upstream-merge-remirror-%d.log", pr.GetNumber())
	ioutil.WriteFile(logpath, cmdoutput, 0600)
	log.Debugf("Wrote log to %s\n", logpath)

	return pr.GetNumber(), nil
}

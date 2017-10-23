# PRMirror
Mirror's pull requests from one repository to another

## Getting started
- This is best off done on a fresh repository as it's likely that you have lots of modified upstream code. We did this for our latest branch https://github.com/HippieStation/HippieStation - We try to maintain modular code by using an additional folder for all of our code: https://github.com/HippieStation/HippieStation/tree/master/hippiestation
- Compile the code by running `go get` and then `go build`
- Copy the following file into your repository [merge-upstream-pull-request.sh](https://github.com/HippieStation/HippieStation/blob/master/hippiestation/tools/merge-upstream-pull-request.sh)
- Make sure that `merge-upstream-pull-request.sh` is marked as executable (`chmod +x merge-upstream-pull-request.sh`)
- Clone the repo to disk
- Make sure that you can push new commits back to the repository from the cloned directory, IE: Setup SSH keys or Github Username/Password
- Run the program to generate a blank config
  - GitHubToken should be a [GitHub Access Token](https://help.github.com/articles/creating-a-personal-access-token-for-the-command-line/)
  - Upstream is at [tgstation/tgstation](https://github.com/tgstation/tgstation/) and Downstream is at [HippieStation/HippieStation](https://github.com/HippieStation/HippieStation/)
    - UpstreamOwner for us is tgstation
    - UpstreamRepo for us is tgstation
    - DownstreamOwner for us is HippieStation
    - DownstreamRepo for us is HippieStation
  - RepoPath is the path to the repository on disk, for us this is: `/home/prmirror/HippieStation/`
  - ToolPath is the path to the tool from within the repository, for us this is: `hippiestation/tools/merge-upstream-pull-request.sh`
  - UseWebhook - should be set to true if you're using the GitHub webhook system instead of scraping the events API
  - WebhookPort - if you're using the webhook system, set the port for the HTTP server to listen on
  - WebhookSecret - if you're using the webhook system, generate a secure secret and set it both on GitHub and in here so we can verify the payloads 
- Make sure before you run the PRMirrorer for the first time that you are 1-1 with your upstream.
- Run the PRMirrorer standalone first to make sure it works, it will open some PRs (just close these, this is a bug which someone could fix), if it polls and is working, continue to set it up as a service and make sure that it doesn't go down
- You're done.


### Current issues:
- On the first run it will open some PRs even though you already have them
- It seems to miss some PRs sometimes, we're working on this - not sure why yet.
- If your server running the bot does down and you don't notice for awhile - you will have issues, it uses the GitHub events API and if the mirrorer is down for too long that the PR merge events are no longer in the event stream you will have an issue - you'll need to do a manual rebase to fix this.

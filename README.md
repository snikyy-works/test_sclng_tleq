# Canvas for Backend Technical Test at Scalingo - Software Engineer Golang

## Description

This software application provides a JSON API capable of performing the following tasks:

- Return data about the last 100 public GitHub repositories.
- Limit results according to various parameters (in this case: language and owner).

To achieve this, I used the GitHub API. Please refer to the "Requirements" section to set up a token for communicating with this API without rate limits.

First, I make a GET request to the API to retrieve the last 100 public GitHub repositories. API_URL: https://api.github.com/repositories?per_page=100&page=1

Then, I have to make a request to each languages_url provided by the repositories to retrieve the number of bytes used by each programming language. It is important to use goroutines to handle 100 requests concurrently, avoiding the inefficiency of making requests one by one.

Once the repository data is in the correct format, we can apply filters. I implemented filtering by **language** and **owner**. The filters are applied based on the parameters received in the request. Additional filters can be easily added by expanding the filterByType function.

Please refer to the "Test" section to know how to use my JSON API.

## Requirements

Setup GITHUB_TOKEN to access GithubAPI without rate limits

- **Step 1** : generate a "Personal Access Tokens" on Github
- **Step 2** : create a .env file in this repository root
- **Step 3** : add the token to the .env file as follows: *GITHUB_TOKEN=token_value*

## Upgrade dependencies in go mod file + vendoring

```bash
go get -u all
go mod tidy
go mod vendor
```

(PS : go version cannot be upgraded, please refer to this error : *github.com/Scalingo/sclng-backend-test-v1: cannot compile Go 1.23 code*)

## Execution

```
docker compose up
```

Application will be then running on port `5000`

## Test

#### Test server is running

```bash
$ curl localhost:5000/ping
# { "status": "pong" }
```

#### Get last 100 public repositories from GithubAPI

```bash
$ curl localhost:5000/repos
# please find the 10/16/24 4pm results in json/all_repos.json 
```

#### Get public repositories from GithubAPI containing a specific programming language

```bash
# curl localhost:5000/repos/lang/{lang}
$ curl localhost:5000/repos/lang/Java
# please find the 10/16/24 4pm results in json/lang.json
```

#### Get public repositories from GithubAPI owned by a specific person

```bash
# curl localhost:5000/repos/owner/{owner}
$ curl localhost:5000/repos/owner/engineyard
# please find the 10/16/24 4pm results in json/owner.json
```

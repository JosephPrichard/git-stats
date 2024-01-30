# GithubStats
A script to get repo metric from a github account such as lines of code or files per language.

Uses HTTP package and wait groups from Go to download zip archives in parallel for the lines of code metric.

Execute the program with `go run main.go`. Configurations should be in a `.env` file in the same directory.
```
token=<your-github-token>
name=JohnSmith
include=ts go rust
exclude=cmake-build-debug target
```
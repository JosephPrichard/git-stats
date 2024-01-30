# GithubStats
A script to get repo metric from a github account such as lines of code or files per language.

Uses HTTP package and channels from Go to download files in parallel for the lines of code metric.
The script contains two solutions to the download the files in parallel.

`1)
The crux of this file download solution is to start N goroutines to download using an HTTP request, each sending the response to a channel.
The channel receives the N responses elsewhere and modifies a map based on the responses.
`

`2)
In this other solution we create a wait group, start N goroutines to download using an HTTP request, and Add to the wait group for each goroutine.
Each goroutine modifies the response map directly, so they lock and unlock a mutex that protects the map.
The wait group waits until each goroutine signals it is done.
`

Execute the program with `go run main.go`. Configurations should be in a `.env` file in the same directory.
```
token=<your-github-token>
name=JohnSmith
include=ts go rust
exclude=cmake-build-debug target
```
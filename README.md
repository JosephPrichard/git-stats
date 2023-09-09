# GithubStats
A script to get repo metric from a github account such as lines of code or files per language.

Uses HTTP package and channels from Go to download files in parallel for the lines of code metric.
The script contains two solutions to the download the files in parallel.

`
1)
The crux of this file download solution is to start N goroutines to download using an HTTP request, each sending the response to a channel.
The channel receives the N responses elsewhere and modifies a map based on the responses.
`
`
2)
In this other solution we create a wait group, start N goroutines to download using an HTTP request, and Add to the wait group for each goroutine.
Each goroutine modifies the response map directly, so they lock and unlock a mutex that protects the map.
The wait group waits until each goroutine signals it is done.
`

The original solution with both a wait group and a channel hung indefinitely because the wait group and channel deadlocked each other. The wait group done call ran after the channel send, but the channel recv ran after the wait group wait call.
I learned that wait groups are better used when you want to wait on N tasks without getting a response, and channels are better if you need each task to send a response when finished.

Execute the program with go run main.go sync.go <your-account-name> <your-github-token>

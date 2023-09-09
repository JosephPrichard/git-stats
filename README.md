# GithubStats
Demonstrates the usage of HTTP and channels from Go to download files in parallel.
The crux of the file download solution is to start N goroutines to download using an HTTP request, each sending the response to a channel and receiving the N messages on the channel elsewhere.
The original solution with a wait group hung indefinitely because the wait group and channel deadlocked each other. The wait group done call ran after the channel send, but the channel recv ran after the wait group wait call.
I learned that wait groups are better used when you want to wait on N tasks without getting a response, and channels are better if you need each task to send a response when finished.

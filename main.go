package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"

	"golang.org/x/exp/slices"
)

type Repo struct {
	Name string `json:"name"`
}

type Node struct {
	Name        string `json:"name"`
	DownloadUrl string `json:"download_url"`
}

type PendingFile struct {
	Ext         string
	DownloadUrl string
}

type CountPair struct {
	Ext        string
	LinesCount int
}

var client = &http.Client{}
var token string = os.Args[2]
var dirExcludeList = []string{".venv", "target"}
var fileIncludeList = []string{"ts", "rs", "go", "java", "kt", "c", "cpp", "js", "cs", "py"}

func main() {
	fmt.Println("Starting the script")

	name := os.Args[1]

	files := []PendingFile{}
	filesCount := map[string]int{}
	linesCount := map[string]int{}
	repos := getRepos(name)

	for _, repo := range repos {
		fmt.Println("Repo ", repo.Name)
		baseUrl := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents", name, repo.Name)
		countRepo(baseUrl, &files)
	}

	for _, file := range files {
		filesCount[file.Ext] += 1
	}
	fmt.Println("\nFiles Count")
	printMap(filesCount, "files")

	fmt.Println("Begin download to count lines")
	var wg sync.WaitGroup
	var ch chan CountPair
	for _, file := range files {
		wg.Add(1)
		go func(file PendingFile) {
			data := get(file.DownloadUrl)
			fmt.Println(file.DownloadUrl)
			ch <- CountPair{
				LinesCount: countLines(string(data)),
				Ext:        file.Ext,
			}
		}(file)
	}
	wg.Wait()

	for pair := range ch {
		linesCount[pair.Ext] += pair.LinesCount
	}
	fmt.Println("\nLines Count")
	printMap(linesCount, "lines")
}

type Pair = struct {
	string
	int
}

func printMap(m map[string]int, metric string) {
	total := 0
	slice := []Pair{}
	for k, v := range m {
		total += v
		slice = append(slice, Pair{k, v})
	}
	sort.Slice(slice, func(i, j int) bool {
		return slice[i].int > slice[j].int
	})

	for _, pair := range slice {
		k := pair.string
		v := pair.int
		percentage := int(float32(v) / float32(total) * 100)
		fmt.Printf("%s \t %d %s \t %d%%\t", k, v, metric, percentage)
		for i := 0; i < percentage; i++ {
			fmt.Print("|")
		}
		fmt.Println()
	}
	fmt.Println()
}

func get(url string) []byte {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		onError(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	// Send the request
	res, err := client.Do(req)
	if err != nil {
		onError(err)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		onError(err)
	}
	// fmt.Println(string(body))
	return body
}

func onError(err error) {
	fmt.Printf("%s\n", err.Error())
	os.Exit(1)
}

func countLines(data string) int {
	return len(strings.Split(data, "\n"))
}

func getRepos(name string) []Repo {
	url := fmt.Sprintf("https://api.github.com/users/%s/repos", name)
	body := get(url)
	repos := []Repo{}
	err := json.Unmarshal(body, &repos)
	if err != nil {
		onError(err)
	}

	return repos
}

func countRepo(url string, files *[]PendingFile) {
	fmt.Println(url)
	body := get(url)

	nodes := []Node{}
	err := json.Unmarshal(body, &nodes)
	if err != nil {
		onError(err)
	}

	for _, node := range nodes {
		tokenized := strings.Split(node.Name, ".")
		ext := tokenized[len(tokenized)-1]
		if node.DownloadUrl != "" {
			// file
			if slices.Contains(fileIncludeList, ext) {
				fmt.Println("File ", node.Name)
				pending := PendingFile{DownloadUrl: node.DownloadUrl, Ext: ext}
				*files = append(*files, pending)
			}
		} else {
			// dir
			nextUrl := fmt.Sprintf("%s/%s", url, node.Name)
			if !slices.Contains(dirExcludeList, node.Name) {
				countRepo(nextUrl, files)
			}
		}
	}
}

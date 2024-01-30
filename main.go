package main

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed .env
var env string

type Config struct {
	client     *http.Client
	name       string
	token      string
	dirExc     []string
	fileIncMap map[string]string
}

func main() {
	fmt.Println("Starting the script")
	start := time.Now()

	var envMap = getEnvVars(env)
	fmt.Printf("EnvMap: %v\n", envMap)

	config := Config{
		client:     &http.Client{},
		name:       envMap["name"],
		token:      envMap["token"],
		dirExc:     strings.Split(envMap["exclude"], " "),
		fileIncMap: toExtMap(envMap["include"]),
	}

	repos := getRepos(config)

	files := make([]FileRecord, 0)
	reposCount := map[string]int{}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		downloadRepos(repos, &files, config)
		wg.Done()
	}()
	go func() {
		countRepos(repos, reposCount, config)
		wg.Done()
	}()
	wg.Wait()

	filesCount := map[string]int{}
	for _, file := range files {
		filesCount[file.Ext] += 1
	}

	linesCount := map[string]int{}
	repoLCount := map[string]int{}
	for _, file := range files {
		linesCount[file.Ext] += file.LinesCount
		repoLCount[file.RepoName] += file.LinesCount
	}

	t := time.Now()
	elapsed := t.Sub(start)

	fmt.Println()
	printMap(filesCount, "files")
	printMap(linesCount, "lines")
	printMap(reposCount, "repos")
	printMap(repoLCount, "lines")

	fmt.Printf("\nScript took %d ms\n", elapsed/time.Millisecond)
}

func toExtMap(includeStr string) map[string]string {
	expMap := make(map[string]string)
	for _, group := range strings.Split(includeStr, " ") {
		for _, ext := range strings.Split(group, "/") {
			expMap[ext] = group
		}
	}
	return expMap
}

func getEnvVars(env string) map[string]string {
	envMap := make(map[string]string)
	for _, line := range strings.Split(env, "\n") {
		line := strings.ReplaceAll(line, "\r", "")
		i := strings.Index(line, "=")
		envMap[line[:i]] = line[i+1:]
	}
	return envMap
}

func getRequest(url string, config Config) []byte {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		onError(err)
	}
	req.Header.Set("Authorization", "Bearer "+config.token)

	// Send the request
	res, err := config.client.Do(req)
	if err != nil {
		onError(err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		onError(err)
	}

	if res.StatusCode != 200 {
		fmt.Printf("Non ok request: %d\n", res.StatusCode)
	}
	return body
}

func onError(err error) {
	fmt.Printf("%s\n", err.Error())
	os.Exit(1)
}

type Repo struct {
	Name string `json:"name"`
}

func getRepos(config Config) []Repo {
	url := fmt.Sprintf("https://api.github.com/users/%s/repos", config.name)
	body := getRequest(url, config)

	repos := make([]Repo, 0)
	err := json.Unmarshal(body, &repos)
	if err != nil {
		onError(err)
	}

	return repos
}

func countRepos(repos []Repo, reposCount map[string]int, config Config) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, repo := range repos {
		wg.Add(1)
		fmt.Printf("Download Repo %s\n", repo.Name)

		go func(repo Repo) {
			defer wg.Done()

			onCount := func(key string) {
				mu.Lock()
				defer mu.Unlock()
				reposCount[key] += 1
			}

			countRepo(repo.Name, onCount, config)
		}(repo)
	}

	wg.Wait()
}

func countRepo(repoName string, onCount func(key string), config Config) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/languages", config.name, repoName)

	body := getRequest(url, config)
	mapRes := map[string]int{}
	err := json.Unmarshal(body, &mapRes)
	if err != nil {
		onError(err)
	}

	lk, lv := "", 0
	for k, v := range mapRes {
		if v > lv {
			lk = k
			lv = v
		}
	}

	onCount(lk)
}

type FileRecord struct {
	Ext        string
	RepoName   string
	LinesCount int
}

func downloadRepos(repos []Repo, files *[]FileRecord, config Config) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, repo := range repos {
		wg.Add(1)
		fmt.Printf("Download Repo %s\n", repo.Name)

		go func(repo Repo) {
			defer wg.Done()

			onFile := func(fr FileRecord) {
				mu.Lock()
				defer mu.Unlock()
				*files = append(*files, fr)
			}

			downloadRepo(repo.Name, onFile, config)
		}(repo)
	}

	wg.Wait()
}

func downloadRepo(repoName string, onFile func(fr FileRecord), config Config) {
	url := fmt.Sprintf("https://github.com/%s/%s/archive/master.zip", config.name, repoName)
	data := getRequest(url, config)

	r := bytes.NewReader(data)
	archive, err := zip.NewReader(r, int64(len(data)))
	if err != nil {
		onError(err)
	}

	for _, zipFile := range archive.File {
		//fmt.Println("Reading file: ", zipFile.Name)
		buf := readZipFile(zipFile)

		// get the extension from the file name
		tokenized := strings.Split(zipFile.Name, ".")
		ext := tokenized[len(tokenized)-1]

		group, ok := config.fileIncMap[ext]
		if ok {
			zfLines := countLines(string(buf))
			fr := FileRecord{
				Ext:        group,
				RepoName:   repoName,
				LinesCount: zfLines,
			}
			onFile(fr)
		}
	}
}

func readZipFile(zf *zip.File) []byte {
	f, err := zf.Open()
	if err != nil {
		onError(err)
	}
	defer f.Close()
	buf, err := io.ReadAll(f)
	if err != nil {
		onError(err)
	}
	return buf
}

func countLines(data string) int {
	count := 0
	for _, line := range strings.Split(data, "\n") {
		if line != "" && line != "\r" {
			count += 1
		}
	}
	return count
}

func printMap(m map[string]int, metric string) {
	type Pair = struct {
		string
		int
	}

	pairs := make([]Pair, 0)

	total := 0
	for k, v := range m {
		total += v
		pairs = append(pairs, Pair{k, v})
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].int > pairs[j].int
	})

	for _, pair := range pairs {
		k := pair.string
		if k == "" {
			k = "<none>"
		}
		v := pair.int

		percentage := int(float32(v) / float32(total) * 100)

		fmt.Printf("%18s %8d %s %5d%% \t", k, v, metric, percentage)
		for i := 0; i < percentage; i++ {
			fmt.Print("|")
		}
		fmt.Println()
	}
	fmt.Println()
}

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

	"github.com/fatih/color"
)

//go:embed config.json
var jsonConfig []byte

type ParsedConfig struct {
	Token   string        `json:"token"`
	Users   []string      `json:"users"`
	Repos   []string      `json:"repos"`
	Include []interface{} `json:"include"`
	Exclude []string      `json:"exclude"`
}

type Config struct {
	client     *http.Client
	token      string
	usernames  []string
	reponames  []string
	includeMap map[string]string
	excludeMap map[string]struct{}
}

func main() {
	fmt.Println("Starting the script")
	start := time.Now()

	var config = parseConfig()
	fmt.Printf("EnvMap: %v\n", config)

	config.client = &http.Client{}

	repos := getRepos(config)
	appendExtraRepos(&repos, config.reponames)

	files, reposCount := createFileRepoTables(repos, config)
	filesCount := createFilesCountTable(files)
	langLinesCount, repoLinesCount, groupedLinesCount := createStatTables(files)

	t := time.Now()
	elapsed := t.Sub(start)

	fmt.Println()
	printTable(filesCount, "files")
	printTable(langLinesCount, "lines")
	printTable(reposCount, "repos")
	printTable(repoLinesCount, "lines")

	for k, v := range groupedLinesCount {
		color.Red("%22s\n", k)
		printTable(v, "lines")
	}

	fmt.Printf("\nScript took %d ms\n", elapsed/time.Millisecond)
}

func parseConfig() Config {
	var parsedConfig ParsedConfig
	err := json.Unmarshal(jsonConfig, &parsedConfig)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}

	config := Config{
		token:      parsedConfig.Token,
		usernames:  parsedConfig.Users,
		reponames:  parsedConfig.Repos,
		excludeMap: map[string]struct{}{},
		includeMap: map[string]string{},
	}

	for _, dir := range parsedConfig.Exclude {
		config.excludeMap[dir] = struct{}{}
	}

	for _, group := range parsedConfig.Include {
		switch x := group.(type) {
		case string:
			config.includeMap[x] = x
		case []string:
			var groupSb strings.Builder
			for i := 0; i < len(x); i++ {
				groupSb.WriteString(x[i])
				groupSb.WriteString("/")
			}
			groupSb.WriteString(x[len(x)-1])

			for _, elem := range x {
				config.includeMap[elem] = groupSb.String()
			}
		}
	}

	return config
}

func getRequest(url string, config Config) []byte {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	req.Header.Set("Authorization", "Bearer "+config.token)

	res, err := config.client.Do(req)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}

	if res.StatusCode != 200 {
		fmt.Printf("Non ok request: %d\n", res.StatusCode)
	}
	return body
}

func appendExtraRepos(repos *[]Repo, repoNames []string) {
	for _, repoName := range repoNames {
		var name strings.Builder
		for _, ch := range repoName {
			if ch == '\\' {
				break
			}
			name.WriteRune(ch)
		}
		repo := Repo{
			Name:     repoName,
			Fullname: repoName,
		}
		*repos = append(*repos, repo)
	}
}

func createFileRepoTables(repos []Repo, config Config) ([]FileRecord, map[string]int) {
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

	return files, reposCount
}

func createFilesCountTable(files []FileRecord) map[string]int {
	filesCount := map[string]int{}
	for _, file := range files {
		filesCount[file.Ext] += 1
	}
	return filesCount
}

func createStatTables(files []FileRecord) (map[string]int, map[string]int, map[string]map[string]int) {
	langLinesCount := map[string]int{}
	repoLinesCount := map[string]int{}
	groupedLinesCount := map[string]map[string]int{}

	for _, file := range files {
		langLinesCount[file.Ext] += file.LinesCount
		repoLinesCount[file.RepoName] += file.LinesCount

		groupLinesCount, ok := groupedLinesCount[file.RepoName]
		if !ok {
			groupLinesCount = map[string]int{}
			groupedLinesCount[file.RepoName] = groupLinesCount
		}
		groupLinesCount[file.Ext] += file.LinesCount
	}

	return langLinesCount, repoLinesCount, groupedLinesCount
}

type Repo struct {
	Name     string `json:"name"`
	Fullname string `json:"full_name"`
}

func getRepos(config Config) []Repo {
	var wg sync.WaitGroup
	var mu sync.Mutex
	repos := make([]Repo, 0)

	for _, user := range config.usernames {
		wg.Add(1)
		go func(user string) {
			defer wg.Done()

			fmt.Printf("Getting repos for user %s\n", user)
			url := fmt.Sprintf("https://api.github.com/users/%s/repos", user)
			body := getRequest(url, config)

			userRepos := make([]Repo, 0)
			err := json.Unmarshal(body, &userRepos)
			if err != nil {
				fmt.Printf("%s\n", err.Error())
				os.Exit(1)
			}

			mu.Lock()
			defer mu.Unlock()
			repos = append(repos, userRepos...)
		}(user)
	}
	wg.Wait()

	return repos
}

func countRepos(repos []Repo, reposCount map[string]int, config Config) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, repo := range repos {
		wg.Add(1)
		fmt.Printf("Counting Repo %s\n", repo.Name)

		go func(repo Repo) {
			defer wg.Done()
			repoKey := findGreatestLangCount(repo, config)

			mu.Lock()
			defer mu.Unlock()
			reposCount[repoKey] += 1
		}(repo)
	}

	wg.Wait()
}

func findGreatestLangCount(repo Repo, config Config) string {
	url := fmt.Sprintf("https://api.github.com/repos/%s/languages", repo.Fullname)

	body := getRequest(url, config)
	mapRes := map[string]int{}
	err := json.Unmarshal(body, &mapRes)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}

	largestKey, largestValue := "", 0
	for key, val := range mapRes {
		if val > largestValue {
			largestKey = key
			largestValue = val
		}
	}

	return largestKey
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
		fmt.Printf("Download Repo %s\n", repo.Fullname)

		go func(repo Repo) {
			defer wg.Done()

			onFile := func(fr FileRecord) {
				mu.Lock()
				defer mu.Unlock()
				*files = append(*files, fr)
			}

			downloadRepo(repo, onFile, config)
		}(repo)
	}

	wg.Wait()
}

func downloadRepo(repo Repo, onFile func(fr FileRecord), config Config) {
	url := fmt.Sprintf("https://github.com/%s/archive/master.zip", repo.Fullname)
	data := getRequest(url, config)

	r := bytes.NewReader(data)
	archive, err := zip.NewReader(r, int64(len(data)))
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}

	for _, zipFile := range archive.File {
		buf := readZipFile(zipFile)

		// get the extension from the file name
		pathTokens := strings.Split(zipFile.Name, ".")
		if len(pathTokens) < 2 {
			fmt.Printf("Needs to have at least 1 path token %s\n", err.Error())
			os.Exit(1)
		}
		ext := pathTokens[len(pathTokens)-1]
		pathTokens = strings.Split(pathTokens[1], "/") // the second element after the first period will be the path

		// skip if any part of the path is contained within the exclude map
		stop := false
		for _, segment := range pathTokens {
			_, exists := config.excludeMap[segment]
			if exists {
				stop = true
				break
			}
		}
		if stop {
			continue
		}

		// check if the extension is a grouping type we want to track
		group, ok := config.includeMap[ext]
		if ok {
			zfLines := countLines(string(buf))
			onFile(FileRecord{
				Ext:        group,
				RepoName:   repo.Name,
				LinesCount: zfLines,
			})
		}
	}
}

func readZipFile(zf *zip.File) []byte {
	f, err := zf.Open()
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	defer f.Close()
	buf, err := io.ReadAll(f)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
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

func printTable(m map[string]int, metric string) {
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

		_, err := color.New(color.FgCyan).Printf("%22s", k)
		if err != nil {
			fmt.Printf("%s\n", err.Error())
			os.Exit(1)
		}
		_, err = color.New(color.FgMagenta).Printf("%8d %s", v, metric)
		if err != nil {
			fmt.Printf("%s\n", err.Error())
			os.Exit(1)
		}
		_, err = color.New(color.FgGreen).Printf("%5d%% \t", percentage)
		if err != nil {
			fmt.Printf("%s\n", err.Error())
			os.Exit(1)
		}
		for i := 0; i < percentage; i++ {
			_, err := color.New(color.BgWhite).Printf(" ")
			if err != nil {
				fmt.Printf("%s\n", err.Error())
				os.Exit(1)
			}
		}
		fmt.Println()
	}
	fmt.Println()
}

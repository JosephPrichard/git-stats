package main

import (
	"fmt"
	"sync"
)

func CountAllLinesCh(linesCount map[string]int, files []PendingFile) {
	fmt.Println("Begin download to count lines")

	ch := make(chan CountPair)
	for _, file := range files {
		go func(file PendingFile) {
			fmt.Println("Start ", file.DownloadUrl)
			data := get(file.DownloadUrl)
			fmt.Println("Finish ", file.DownloadUrl)
			ch <- CountPair{
				LinesCount: countLines(string(data)),
				Ext:        file.Ext,
			}
		}(file)
	}

	for i := 0; i < len(files); i++ {
		pair := <-ch
		fmt.Println("Channel", pair.Ext, pair.LinesCount)
		linesCount[pair.Ext] += pair.LinesCount
	}
}

func CountAllLinesWg(linesCount map[string]int, files []PendingFile) {
	fmt.Println("Begin download to count lines")

	var wg sync.WaitGroup
	var m sync.Mutex
	for _, file := range files {
		wg.Add(1)
		go func(file PendingFile) {
			defer wg.Done()
			fmt.Println("Start ", file.DownloadUrl)
			data := get(file.DownloadUrl)
			m.Lock()
			linesCount[file.Ext] += countLines(string(data))
			m.Unlock()
			fmt.Println("Finish ", file.DownloadUrl)
		}(file)
	}

	wg.Wait()
}


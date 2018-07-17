package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"golang.org/x/net/proxy"
	"golang.org/x/time/rate"
)

var (
	pt = fmt.Printf
)

func main() {
	dir := "/media/videos/bt"
	config := torrent.NewDefaultClientConfig()
	config.DataDir = dir
	config.UploadRateLimiter = rate.NewLimiter(
		rate.Every(time.Second*1),
		1024*32,
	)
	config.TrackerHttpClient = func() *http.Client {
		dialer, err := proxy.SOCKS5(
			"tcp",
			"10.0.0.1:1080",
			nil,
			proxy.Direct,
		)
		if err != nil {
			panic(err)
		}
		client := &http.Client{
			Transport: &http.Transport{
				Dial: dialer.Dial,
			},
		}
		return client
	}()
	client, err := torrent.NewClient(config)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	fileSet := new(sync.Map)
	addTorrent := func() {
		torrentFiles, err := filepath.Glob(filepath.Join(dir, "*.torrent"))
		if err != nil {
			panic(err)
		}
		for _, torrentFile := range torrentFiles {
			if _, ok := fileSet.Load(torrentFile); ok {
				continue
			}
			fileSet.Store(torrentFile, true)
			torrentFile := torrentFile
			pt("%s\n", torrentFile)
			go func() {
				t, err := client.AddTorrentFromFile(torrentFile)
				if err != nil {
					panic(err)
				}
				<-t.GotInfo()
				t.DownloadAll()
				for range time.NewTicker(time.Second * 10).C {
					stats := t.Stats()
					pt(
						"%s: <downloaded %5.2f%%> <peers %d/%d/%d/%d/%d> <file %s>\n",
						time.Now().Format("15:04:05"),
						float64(t.BytesCompleted())/float64(t.Length())*100,
						stats.PendingPeers,
						stats.HalfOpenPeers,
						stats.ConnectedSeeders,
						stats.ActivePeers,
						stats.TotalPeers,
						torrentFile,
					)
					if t.BytesCompleted() == t.Length() {
						//if err := os.Rename(torrentFile, torrentFile+".complete"); err != nil {
						//	panic(err)
						//}
						//fileSet.Delete(fileSet)
						break
					}
				}
			}()
		}
	}

	addTorrent()
	go func() {
		for range time.NewTicker(time.Second * 3).C {
			addTorrent()
		}
	}()

	select {}
}

package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/bencode"
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
	config.ProxyURL = "socks5://10.0.0.1:1080"
	client, err := torrent.NewClient(config)
	if err != nil {
		panic(err)
	}
	defer func() {
		client.Close()
		pt("closed\n")
	}()

	if len(os.Args) > 1 {

		link := os.Args[1]
		t, err := client.AddMagnet(link)
		if err != nil {
			panic(err)
		}
		pt("getting info..\n")
		<-t.GotInfo()
		pt("ok\n")
		info := t.Info()
		t.Drop()
		info.Name = ""
		f, err := os.Create("foo.torrent")
		if err != nil {
			panic(err)
		}
		if err := bencode.NewEncoder(f).Encode(info); err != nil {
			panic(err)
		}
		if err := f.Close(); err != nil {
			panic(err)
		}

		return
	}

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)

	fileSet := new(sync.Map)
	magnetSet := new(sync.Map)
	addTorrent := func() {

		// torrent files
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
					ps := t.PieceStateRuns()
					var numPartial, numChecking, numComplete int
					for _, p := range ps {
						if p.Partial {
							numPartial += p.Length
						}
						if p.Checking {
							numChecking += p.Length
						}
						if p.Complete {
							numComplete += p.Length
						}
					}
					stats := t.Stats()
					pt(
						"%s: <downloaded %5.2f%%> <peers %d/%d/%d/%d/%d> <piece %d/%d/%d/%d> <file %s>\n",
						time.Now().Format("15:04:05"),
						float64(t.BytesCompleted())/float64(t.Length())*100,
						stats.PendingPeers,
						stats.HalfOpenPeers,
						stats.ConnectedSeeders,
						stats.ActivePeers,
						stats.TotalPeers,
						numPartial,
						numChecking,
						numComplete,
						t.NumPieces(),
						torrentFile,
					)
					if t.BytesCompleted() == t.Length() {
						if err := os.Rename(torrentFile, torrentFile+".complete"); err != nil {
							panic(err)
						}
						//if err := os.Remove(torrentFile); err != nil {
						//	panic(err)
						//}
						fileSet.Delete(fileSet)
						t.Drop()
						break
					}
				}
			}()
		}

		// magnet links
		magnets, err := filepath.Glob(filepath.Join(dir, "magnet:?*"))
		if err != nil {
			panic(err)
		}
		for _, magnet := range magnets {
			magnet := magnet
			link := strings.TrimPrefix(magnet, dir)
			link = strings.TrimPrefix(link, "/")
			if _, ok := magnetSet.Load(link); ok {
				continue
			}
			magnetSet.Store(link, true)
			go func() {
				pt("%s\n", link)
				t, err := client.AddMagnet(link)
				if err != nil {
					panic(err)
				}
				<-t.GotInfo()
				metaInfo := t.Metainfo()
				t.Drop()
				f, err := os.Create(filepath.Join(dir, t.Info().Name+".torrent"))
				if err != nil {
					panic(err)
				}
				if err := bencode.NewEncoder(f).Encode(metaInfo); err != nil {
					panic(err)
				}
				if err := f.Close(); err != nil {
					panic(err)
				}
				if err := os.Remove(magnet); err != nil {
					panic(err)
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

	select {
	case <-c:
	}
}

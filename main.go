package main

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/bencode"
	"github.com/reusee/e/v2"
	"golang.org/x/time/rate"
)

var (
	me     = e.Default.WithStack()
	ce, he = e.New(me)
	pt     = fmt.Printf
)

func main() {
	dir := "."
	config := torrent.NewDefaultClientConfig()
	config.DataDir = dir
	config.UploadRateLimiter = rate.NewLimiter(
		rate.Every(time.Second*1),
		1024*32,
	)
	config.ProxyURL = "socks5://10.0.0.3:1080"
	peerID, err := hex.DecodeString("2d4754303030322d308b23248a2bbbfe67be28c0")
	ce(err)
	config.PeerID = string(peerID)
	client, err := torrent.NewClient(config)
	ce(err)
	defer func() {
		client.Close()
		pt("closed\n")
	}()
	pt("peer id: %x\n", client.PeerID())

	if len(os.Args) > 1 {

		link := os.Args[1]
		t, err := client.AddMagnet(link)
		ce(err)
		pt("getting info..\n")
		<-t.GotInfo()
		pt("ok\n")
		info := t.Info()
		t.Drop()
		info.Name = ""
		f, err := os.Create("foo.torrent")
		ce(err)
		ce(bencode.NewEncoder(f).Encode(info))
		ce(f.Close())

		return
	}

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)

	fileSet := new(sync.Map)
	magnetSet := new(sync.Map)
	addTorrent := func() {

		// torrent files
		torrentFiles, err := filepath.Glob(filepath.Join(dir, "*.torrent"))
		ce(err)
		for _, torrentFile := range torrentFiles {
			if _, ok := fileSet.Load(torrentFile); ok {
				continue
			}
			fileSet.Store(torrentFile, true)
			torrentFile := torrentFile
			pt("%s\n", torrentFile)
			go func() {
				t, err := client.AddTorrentFromFile(torrentFile)
				ce(err)
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
						ce(os.Rename(torrentFile, torrentFile+".complete"))
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
		addMagnetLink := func(link string) {
			if _, ok := magnetSet.Load(link); ok {
				return
			}
			magnetSet.Store(link, true)
			go func() {
				pt("%s\n", link)
				t, err := client.AddMagnet(link)
				ce(err)
				<-t.GotInfo()
				metaInfo := t.Metainfo()
				t.Drop()
				f, err := os.Create(filepath.Join(dir, t.Info().Name+".torrent"))
				ce(err)
				ce(bencode.NewEncoder(f).Encode(metaInfo))
				ce(f.Close())
			}()
		}
		// file name
		magnets, err := filepath.Glob(filepath.Join(dir, "magnet:?*"))
		ce(err)
		for _, magnet := range magnets {
			magnet := magnet
			link := strings.TrimPrefix(magnet, dir)
			link = strings.TrimPrefix(link, "/")
			addMagnetLink(link)
			ce(os.Remove(magnet))
		}
		// link in files
		names, err := filepath.Glob(filepath.Join(dir, "*.magnet"))
		ce(err)
		for _, name := range names {
			content, err := ioutil.ReadFile(name)
			ce(err)
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if !strings.HasPrefix(line, "magnet") {
					continue
				}
				addMagnetLink(line)
			}
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

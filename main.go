package main

import (
	"embed"
	"encoding/hex"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/anacrolix/torrent"
	"golang.org/x/time/rate"
)

func main() {
	dir := "."
	config := torrent.NewDefaultClientConfig()
	config.DisableAggressiveUpload = true
	config.DisableIPv6 = true
	config.DataDir = dir
	config.UploadRateLimiter = rate.NewLimiter(
		rate.Every(time.Second*1),
		1024*32,
	)
	proxyURL, err := url.Parse("socks5://localhost:10000")
	ce(err)
	config.HTTPProxy = http.ProxyURL(proxyURL)
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

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)

	var lock sync.Mutex

	fileSet := new(sync.Map)
	addJobs := func() {

		dirFile, err := os.Open(dir)
		ce(err)
		names, err := dirFile.Readdirnames(-1)
		ce(err)

		for _, name := range names {
			path := filepath.Join(dir, name)
			if _, ok := fileSet.Load(path); ok {
				continue
			}
			fileSet.Store(path, true)

			go func() {

				var t *torrent.Torrent
				if strings.HasPrefix(path, "magnet:") {
					pt("add %s\n", path)
					spec, err := torrent.TorrentSpecFromMagnetUri(path)
					ce(err)
					t, _, err = client.AddTorrentSpec(spec)
					ce(err)
				} else if strings.HasSuffix(path, ".torrent") {
					pt("add %s\n", path)
					t, err = client.AddTorrentFromFile(path)
					ce(err)
				} else {
					return
				}

				t.AddTrackers(trackers)
				// load tracker list
				go func() {
					for {
						for _, addr := range []string{
							"https://trackerslist.com/best.txt",
							"https://raw.githubusercontent.com/ngosang/trackerslist/master/trackers_best.txt",
						} {
							func() {
								resp, err := proxyHTTPClient.Get(addr)
								if err != nil {
									pt("%s\n", err)
									return
								}
								defer resp.Body.Close()
								content, err := io.ReadAll(resp.Body)
								if err != nil {
									return
								}
								text := string(content)
								ce(err)
								var trackers [][]string
								for _, line := range strings.Split(text, "\n") {
									line = strings.TrimSpace(line)
									if len(line) == 0 {
										continue
									}
									trackers = append(trackers, []string{line})
								}
								t.AddTrackers(trackers)
								pt("load %d trackers from %s\n", len(trackers), addr)
							}()
						}
						time.Sleep(time.Minute * 30)
					}
				}()

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
					lock.Lock()
					pt("%s: %s\n",
						time.Now().Format("15:04:05"),
						path,
					)
					pt("\t%.2f%% completed\n",
						float64(t.BytesCompleted())/float64(t.Length())*100,
					)
					pt("\tpeers: %d pending, %d half open, %d connected, %d active, %d total\n",
						stats.PendingPeers,
						stats.HalfOpenPeers,
						stats.ConnectedSeeders,
						stats.ActivePeers,
						stats.TotalPeers,
					)
					pt("\tpieces: %d partial, %d checking, %d completed, %d total\n",
						numPartial,
						numChecking,
						numComplete,
						t.NumPieces(),
					)
					lock.Unlock()
					if t.BytesCompleted() == t.Length() {
						ce(os.Rename(path, path+".complete"))
						//if err := os.Remove(torrentFile); err != nil {
						//	panic(err)
						//}
						fileSet.Delete(fileSet)
						t.Drop()
						break
					}
					//client.WriteStatus(os.Stdout)
				}
			}()
		}

	}

	addJobs()
	go func() {
		for range time.NewTicker(time.Second * 3).C {
			addJobs()
		}
	}()

	select {
	case <-c:
	}
}

//go:embed *.txt
var trackerFiles embed.FS

// https://github.com/XIU2/TrackersListCollection/blob/master/README-ZH.md
// https://github.com/ngosang/trackerslist

var trackers = func() (ret [][]string) {
	fs.WalkDir(trackerFiles, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		content, err := fs.ReadFile(trackerFiles, path)
		text := string(content)
		ce(err)
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if len(line) == 0 {
				continue
			}
			ret = append(ret, []string{line})
		}
		return nil
	})
	pt("load %d trackers\n", len(ret))
	return
}()

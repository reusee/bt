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
	"github.com/reusee/e4"
	"golang.org/x/time/rate"
)

var (
	we = e4.DefaultWrap
	ce = e4.Check
	he = e4.Handle
	pt = fmt.Printf
)

func main() {
	dir := "."
	config := torrent.NewDefaultClientConfig()
	config.DataDir = dir
	config.UploadRateLimiter = rate.NewLimiter(
		rate.Every(time.Second*1),
		1024*32,
	)
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
				t.AddTrackers(trackers)
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
					client.WriteStatus(os.Stdout)
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
				t.AddTrackers(trackers)
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

var trackers = func() (ret [][]string) {
	// from https://github.com/ngosang/trackerslist
	const text = `
udp://tracker.coppersurfer.tk:6969/announce

udp://tracker.open-internet.nl:6969/announce

udp://tracker.leechers-paradise.org:6969/announce

udp://tracker.opentrackr.org:1337/announce

udp://tracker.internetwarriors.net:1337/announce

http://tracker.opentrackr.org:1337/announce

http://tracker.internetwarriors.net:1337/announce

udp://9.rarbg.to:2710/announce

udp://9.rarbg.me:2710/announce

udp://tracker.openbittorrent.com:80/announce

http://tracker3.itzmx.com:6961/announce

http://tracker1.itzmx.com:8080/announce

udp://exodus.desync.com:6969/announce

udp://tracker.torrent.eu.org:451/announce

udp://tracker.tiny-vps.com:6969/announce

udp://retracker.lanta-net.ru:2710/announce

udp://tracker2.itzmx.com:6961/announce

udp://tracker.cyberia.is:6969/announce

udp://open.stealth.si:80/announce

udp://open.demonii.si:1337/announce

udp://denis.stalker.upeer.me:6969/announce

http://tracker2.itzmx.com:6961/announce

udp://explodie.org:6969/announce

udp://bt.xxx-tracker.com:2710/announce

http://open.acgnxtracker.com:80/announce

http://explodie.org:6969/announce

udp://tracker4.itzmx.com:2710/announce

http://retracker.mgts.by:80/announce

udp://tracker.moeking.me:6969/announce

udp://torrentclub.tech:6969/announce

udp://ipv4.tracker.harry.lu:80/announce

http://torrentclub.tech:6969/announce

udp://tracker.uw0.xyz:6969/announce

udp://tracker.iamhansen.xyz:2000/announce

udp://zephir.monocul.us:6969/announce

udp://tracker.tvunderground.org.ru:3218/announce

udp://tracker.trackton.ga:7070/announce

udp://tracker.supertracker.net:1337/announce

udp://tracker.nyaa.uk:6969/announce

udp://tracker.lelux.fi:6969/announce

udp://tracker.kamigami.org:2710/announce

udp://tracker.filepit.to:6969/announce

udp://tracker.filemail.com:6969/announce

udp://tracker.dler.org:6969/announce

udp://tracker-udp.gbitt.info:80/announce

udp://retracker.sevstar.net:2710/announce

udp://retracker.netbynet.ru:2710/announce

udp://retracker.maxnet.ua:80/announce

udp://retracker.baikal-telecom.net:2710/announce

udp://retracker.akado-ural.ru:80/announce

udp://newtoncity.org:6969/announce

udp://chihaya.toss.li:9696/announce

udp://bt.dy20188.com:80/announce

https://tracker.vectahosting.eu:2053/announce

https://tracker.lelux.fi:443/announce

https://tracker.gbitt.info:443/announce

https://tracker.fastdownload.xyz:443/announce

https://t.quic.ws:443/announce

https://opentracker.co:443/announce

http://vps02.net.orel.ru:80/announce

http://tracker01.loveapp.com:6789/announce

http://tracker.tvunderground.org.ru:3218/announce

http://tracker.torrentyorg.pl:80/announce

http://tracker.lelux.fi:80/announce

http://tracker.gbitt.info:80/announce

http://tracker.bz:80/announce

http://torrent.nwps.ws:80/announce

http://t.nyaatracker.com:80/announce

http://retracker.sevstar.net:2710/announce

http://open.trackerlist.xyz:80/announce

http://open.acgtracker.com:1096/announce

http://newtoncity.org:6969/announce

http://gwp2-v19.rinet.ru:80/announce

udp://tracker.msm8916.com:6969/announce

https://tracker.publictorrent.net:443/announce

https://1337.abcvg.info:443/announce

http://tracker4.itzmx.com:2710/announce

http://tracker.publictorrent.net:80/announce

http://tracker.bt4g.com:2095/announce

http://t.acg.rip:6699/announce

http://sub4all.org:2710/announce

http://share.camoe.cn:8080/announce

http://bt-tracker.gamexp.ru:2710/announce

http://agusiq-torrents.pl:6969/announce

  `
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		ret = append(ret, []string{line})
	}
	return
}()

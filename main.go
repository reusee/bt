package main

import (
	"encoding/hex"
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
	proxyURL, err := url.Parse("socks5://192.168.88.1:9103")
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
						path,
					)
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

var trackers = func() (ret [][]string) {
	// from https://github.com/ngosang/trackerslist
	const text = `
	udp://p4p.arenabg.ch:1337/announce

http://p4p.arenabg.com:1337/announce

udp://tracker.internetwarriors.net:1337/announce

http://tracker.internetwarriors.net:1337/announce

udp://tracker.opentrackr.org:1337/announce

http://tracker.opentrackr.org:1337/announce

udp://tracker.openbittorrent.com:6969/announce

udp://exodus.desync.com:6969/announce

udp://www.torrent.eu.org:451/announce

udp://tracker.torrent.eu.org:451/announce

udp://tracker.tiny-vps.com:6969/announce

udp://retracker.lanta-net.ru:2710/announce

udp://open.stealth.si:80/announce

udp://zephir.monocul.us:6969/announce

udp://wassermann.online:6969/announce

udp://vibe.community:6969/announce

udp://valakas.rollo.dnsabr.com:2710/announce

udp://udp-tracker.shittyurl.org:6969/announce

udp://tracker2.dler.org:80/announce

udp://tracker1.bt.moack.co.kr:80/announce

udp://tracker0.ufibox.com:6969/announce

udp://tracker.zerobytes.xyz:1337/announce

udp://tracker.zemoj.com:6969/announce

udp://tracker.uw0.xyz:6969/announce

udp://tracker.theoks.net:6969/announce

udp://tracker.shkinev.me:6969/announce

udp://tracker.ololosh.space:6969/announce

udp://tracker.nrx.me:6969/announce

udp://tracker.monitorit4.me:6969/announce

udp://tracker.moeking.me:6969/announce

udp://tracker.loadbt.com:6969/announce

udp://tracker.lelux.fi:6969/announce

udp://tracker.ccp.ovh:6969/announce

udp://tracker.breizh.pm:6969/announce

udp://tracker.blacksparrowmedia.net:6969/announce

udp://tracker.army:6969/announce

udp://tracker.altrosky.nl:6969/announce

udp://tracker.0x.tf:6969/announce

udp://tracker-de.ololosh.space:6969/announce

udp://tr.cili001.com:8070/announce

udp://torrentclub.online:54123/announce

udp://t3.leech.ie:1337/announce

udp://t2.leech.ie:1337/announce

udp://t1.leech.ie:1337/announce

udp://retracker.sevstar.net:2710/announce

udp://retracker.netbynet.ru:2710/announce

udp://public.publictracker.xyz:6969/announce

udp://public-tracker.zooki.xyz:6969/announce

udp://opentracker.i2p.rocks:6969/announce

udp://opentor.org:2710/announce

udp://open.publictracker.xyz:6969/announce

udp://mts.tvbit.co:6969/announce

udp://movies.zsw.ca:6969/announce

udp://mail.realliferpg.de:6969/announce

udp://ipv4.tracker.harry.lu:80/announce

udp://inferno.demonoid.is:3391/announce

udp://fe.dealclub.de:6969/announce

udp://explodie.org:6969/announce

udp://engplus.ru:6969/announce

udp://edu.uifr.ru:6969/announce

udp://drumkitx.com:6969/announce

udp://discord.heihachi.pw:6969/announce

udp://cutiegirl.ru:6969/announce

udp://code2chicken.nl:6969/announce

udp://camera.lei001.com:6969/announce

udp://bubu.mapfactor.com:6969/announce

udp://bt2.archive.org:6969/announce

udp://bt1.archive.org:6969/announce

udp://bclearning.top:6969/announce

udp://app.icon256.com:8000/announce

udp://admin.videoenpoche.info:6969/announce

udp://6ahddutb1ucc3cp.ru:6969/announce

https://trakx.herokuapp.com:443/announce

https://tracker.tamersunion.org:443/announce

https://tracker.sloppyta.co:443/announce

https://tracker.nitrix.me:443/announce

https://tracker.nanoha.org:443/announce

https://tracker.lelux.fi:443/announce

https://tracker.iriseden.fr:443/announce

https://tracker.iriseden.eu:443/announce

https://tracker.foreverpirates.co:443/announce

https://tracker.coalition.space:443/announce

http://vps02.net.orel.ru:80/announce

http://trk.publictracker.xyz:6969/announce

http://tracker1.bt.moack.co.kr:80/announce

http://tracker.zerobytes.xyz:1337/announce

http://tracker.sloppyta.co:80/announce

http://tracker.noobsubs.net:80/announce

http://tracker.loadbt.com:6969/announce

http://tracker.lelux.fi:80/announce

http://tracker.dler.org:6969/announce

http://tracker.ccp.ovh:6969/announce

http://tracker.breizh.pm:6969/announce

http://tracker-cdn.moeking.me:2095/announce

http://torrenttracker.nwc.acsalaska.net:6969/announce

http://torrentclub.online:54123/announce

http://t.overflow.biz:6969/announce

http://t.nyaatracker.com:80/announce

http://t.acg.rip:6699/announce

http://rt.tace.ru:80/announce

http://retracker.sevstar.net:2710/announce

http://retracker.joxnet.ru:80/announce

http://pow7.com:80/announce

http://opentracker.i2p.rocks:6969/announce

http://open.acgtracker.com:1096/announce

http://open.acgnxtracker.com:80/announce

http://ns3107607.ip-54-36-126.eu:6969/announce

http://h4.trakx.nibba.trade:80/announce

http://explodie.org:6969/announce

udp://vibe.sleepyinternetfun.xyz:1738/announce

udp://tracker4.itzmx.com:2710/announce

udp://tracker.nighthawk.pw:2052/announce

udp://tracker.kali.org:6969/announce

udp://tracker.dler.org:6969/announce

udp://tracker-udp.gbitt.info:80/announce

udp://tr2.ysagin.top:2710/announce

udp://tr.bangumi.moe:6969/announce

udp://concen.org:6969/announce

udp://anidex.moe:6969/announce

https://tr.ready4.icu:443/announce

https://1337.abcvg.info:443/announce

http://tracker4.itzmx.com:2710/announce

http://tracker2.dler.org:80/announce

http://tracker.nighthawk.pw:2052/announce

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

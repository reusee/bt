package main

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
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
	proxyURL, err := url.Parse("socks5://192.168.88.200:9103")
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
	magnetSet := new(sync.Map)
	addTorrent := func() {

		// torrent files
		torrentFiles, err := filepath.Glob(filepath.Join(dir, "*"))
		ce(err)
		for _, torrentFile := range torrentFiles {
			if _, ok := fileSet.Load(torrentFile); ok {
				continue
			}
			fileSet.Store(torrentFile, true)

			torrentFile := torrentFile
			pt("%s\n", torrentFile)
			go func() {

				var t *torrent.Torrent
				if strings.HasPrefix(torrentFile, "magnet:") {
					spec, err := torrent.TorrentSpecFromMagnetUri(torrentFile)
					ce(err)
					t, _, err = client.AddTorrentSpec(spec)
					ce(err)
					pt("add %s\n", torrentFile)
				} else if strings.HasSuffix(torrentFile, ".torrent") {
					t, err = client.AddTorrentFromFile(torrentFile)
					ce(err)
					pt("add %s\n", torrentFile)
				} else {
					pt("skip %s\n", torrentFile)
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
  http://tracker.opentrackr.org:1337/announce

udp://tracker.opentrackr.org:1337/announce

udp://9.rarbg.to:2710/announce

udp://9.rarbg.me:2710/announce

udp://3rt.tace.ru:60889/announce

http://5rt.tace.ru:60889/announce

udp://tracker.internetwarriors.net:1337/announce

http://tracker.internetwarriors.net:1337/announce

udp://tracker.cyberia.is:6969/announce

udp://exodus.desync.com:6969/announce

udp://explodie.org:6969/announce

http://explodie.org:6969/announce

udp://tracker3.itzmx.com:6961/announce

http://tracker3.itzmx.com:6961/announce

http://tracker1.itzmx.com:8080/announce

udp://www.torrent.eu.org:451/announce

udp://tracker.torrent.eu.org:451/announce

udp://open.stealth.si:80/announce

udp://tracker.ds.is:6969/announce

udp://retracker.lanta-net.ru:2710/announce

udp://tracker.tiny-vps.com:6969/announce

http://open.acgnxtracker.com:80/announce

udp://tracker.zerobytes.xyz:1337/announce

udp://tracker.moeking.me:6969/announce

udp://open.demonii.si:1337/announce

http://tracker.zerobytes.xyz:1337/announce

udp://ipv4.tracker.harry.lu:80/announce

http://rt.tace.ru:80/announce

udp://cdn-2.gamecoast.org:6969/announce

udp://cdn-1.gamecoast.org:6969/announce

http://tracker-cdn.moeking.me:2095/announce

udp://valakas.rollo.dnsabr.com:2710/announce

udp://tracker.shkinev.me:6969/announce

udp://t1.leech.ie:1337/announce

udp://opentor.org:2710/announce

udp://mts.tvbit.co:6969/announce

udp://ln.mtahost.co:6969/announce

udp://47.ip-51-68-199.eu:6969/announce

https://trakx.herokuapp.com:443/announce

http://t.overflow.biz:6969/announce

http://opentracker.i2p.rocks:6969/announce

http://h4.trakx.nibba.trade:80/announce

udp://vibe.community:6969/announce

udp://us-tracker.publictracker.xyz:6969/announce

udp://udp-tracker.shittyurl.org:6969/announce

udp://u.wwwww.wtf:1/announce

udp://tracker2.itzmx.com:6961/announce

udp://tracker1.bt.moack.co.kr:80/announce

udp://tracker0.ufibox.com:6969/announce

udp://tracker.zum.bi:6969/announce

udp://tracker.v6speed.org:6969/announce

udp://tracker.uw0.xyz:6969/announce

udp://tracker.sigterm.xyz:6969/announce

udp://tracker.lelux.fi:6969/announce

udp://tracker.army:6969/announce

udp://tracker.altrosky.nl:6969/announce

udp://tracker.0x.tf:6969/announce

udp://torrentclub.online:54123/announce

udp://t3.leech.ie:1337/announce

udp://t2.leech.ie:1337/announce

udp://storage.groupees.com:6969/announce

udp://sd-161673.dedibox.fr:6969/announce

udp://opentracker.i2p.rocks:6969/announce

udp://nagios.tks.sumy.ua:80/announce

udp://movies.zsw.ca:6969/announce

udp://mail.realliferpg.de:6969/announce

udp://johnrosen1.com:6969/announce

udp://inferno.demonoid.is:3391/announce

udp://free-tracker.zooki.xyz:6969/announce

udp://fe.dealclub.de:6969/announce

udp://engplus.ru:6969/announce

udp://edu.uifr.ru:6969/announce

udp://discord.heihachi.pw:6969/announce

udp://daveking.com:6969/announce

udp://code2chicken.nl:6969/announce

udp://bt2.archive.org:6969/announce

udp://bt2.3kb.xyz:6969/announce

udp://bt1.archive.org:6969/announce

udp://bms-hosxp.com:6969/announce

udp://blokas.io:6969/announce

udp://aruacfilmes.com.br:6969/announce

udp://aaa.army:8866/announce

https://w.wwwww.wtf:443/announce

https://tracker.lelux.fi:443/announce

https://aaa.army:8866/announce

http://vps02.net.orel.ru:80/announce

http://tracker2.itzmx.com:6961/announce

http://tracker1.bt.moack.co.kr:80/announce

http://tracker.zum.bi:6969/announce

http://tracker.vraphim.com:6969/announce

http://tracker.lelux.fi:80/announce

http://torrentclub.online:54123/announce

http://aaa.army:8866/announce

udp://tracker4.itzmx.com:2710/announce

udp://cutiegirl.ru:6969/announce

http://00.mercax.com:443/announce

http://tracker2.dler.org:80/announce

udp://tracker2.dler.org:80/announce

udp://tracker.zemoj.com:6969/announce

udp://tracker.teambelgium.net:6969/announce

udp://tracker.skyts.net:6969/announce

udp://tracker.kamigami.org:2710/announce

udp://tracker.fortu.io:6969/announce

udp://tracker.dler.org:6969/announce

udp://tr2.ysagin.top:2710/announce

udp://tr.cili001.com:8070/announce

udp://teamspeak.value-wolf.org:6969/announce

udp://retracker.sevstar.net:2710/announce

udp://retracker.netbynet.ru:2710/announce

udp://public.publictracker.xyz:6969/announce

udp://public-tracker.zooki.xyz:6969/announce

udp://line-net.ru:6969/announce

udp://drumkitx.com:6969/announce

udp://dpiui.reedlan.com:6969/announce

udp://camera.lei001.com:6969/announce

udp://bubu.mapfactor.com:6969/announce

udp://bt.okmp3.ru:2710/announce

udp://bioquantum.co.za:6969/announce

udp://admin.videoenpoche.info:6969/announce

https://tracker.tamersunion.org:443/announce

https://tracker.sloppyta.co:443/announce

https://tracker.nitrix.me:443/announce

https://tracker.nanoha.org:443/announce

https://tracker.imgoingto.icu:443/announce

https://tracker.hama3.net:443/announce

https://tracker.cyber-hub.net:443/announce

https://tracker.cyber-hub.net/announce

https://1337.abcvg.info:443/announce

http://vpn.flying-datacenter.de:6969/announce

http://tracker.sloppyta.co:80/announce

http://tracker.skyts.net:6969/announce

http://tracker.noobsubs.net:80/announce

http://tracker.kamigami.org:2710/announce

http://tracker.dler.org:6969/announce

http://torrenttracker.nwc.acsalaska.net:6969/announce

http://t.nyaatracker.com:80/announce

http://t.acg.rip:6699/announce

http://retracker.sevstar.net:2710/announce

http://open.acgtracker.com:1096/announce

http://bt.okmp3.ru:2710/announce

http://bobbialbano.com:6969/announce

udp://tsundere.pw:6969/announce

udp://tracker.kali.org:6969/announce

udp://tracker.filemail.com:6969/announce

udp://tr.bangumi.moe:6969/announce

udp://qg.lorzl.gq:2710/announce

udp://open.lolicon.eu:7777/announce

udp://ns389251.ovh.net:6969/announce

udp://ns-1.x-fins.com:6969/announce

udp://concen.org:6969/announce

udp://bt2.54new.com:8080/announce

udp://bt.firebit.org:2710/announce

udp://anidex.moe:6969/announce

https://tracker.foreverpirates.co:443/announce

https://tracker.coalition.space:443/announce

http://tracker4.itzmx.com:2710/announce

http://tracker.bt4g.com:2095/announce



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

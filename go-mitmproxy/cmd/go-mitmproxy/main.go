package main

import (
	logger "Keydd/log"
	"fmt"
	rawLog "log"
	"net/http"
	"os"

	"Keydd/go-mitmproxy/addon"
	"Keydd/go-mitmproxy/internal/helper"
	"Keydd/go-mitmproxy/proxy"
	"Keydd/go-mitmproxy/web"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	version bool // show go-mitmproxy version

	Addr         string   // proxy listen addr
	WebAddr      string   // web interface listen addr
	SslInsecure  bool     // not verify upstream server SSL/TLS certificates.
	IgnoreHosts  []string // a list of ignore hosts
	AllowHosts   []string // a list of allow hosts
	CertPath     string   // path of generate cert files
	Debug        int      // debug mode: 1 - print debug log, 2 - show debug from
	Dump         string   // dump filename
	DumpLevel    int      // dump level: 0 - header, 1 - header + body
	Upstream     string   // upstream proxy
	UpstreamCert bool     // Connect to upstream server to look up certificate details. Default: True
	MapRemote    string   // map remote config filename
	MapLocal     string   // map local config filename

	filename string // read config from the filename
}

func main() {
	config := loadConfig()

	if config.Debug > 0 {
		rawLog.SetFlags(rawLog.LstdFlags | rawLog.Lshortfile)
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	if config.Debug == 2 {
		log.SetReportCaller(true)
	}
	//log.SetOutput(os.Stdout)
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		panic(err)
	}
	log.SetOutput(null)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	opts := &proxy.Options{
		Debug:             config.Debug,
		Addr:              config.Addr,
		StreamLargeBodies: 1024 * 1024 * 5,
		SslInsecure:       config.SslInsecure,
		CaRootPath:        config.CertPath,
		Upstream:          config.Upstream,
	}

	p, err := proxy.NewProxy(opts)
	if err != nil {
		log.Fatal(err)
	}

	if config.version {
		fmt.Println("go-mitmproxy: " + p.Version)
		os.Exit(0)
	}

	logger.Info.Println("go-mitmproxy version %v\n", p.Version)

	if len(config.IgnoreHosts) > 0 {
		p.SetShouldInterceptRule(func(req *http.Request) bool {
			return !helper.MatchHost(req.Host, config.IgnoreHosts)
		})
	}
	if len(config.AllowHosts) > 0 {
		p.SetShouldInterceptRule(func(req *http.Request) bool {
			return helper.MatchHost(req.Host, config.AllowHosts)
		})
	}

	if !config.UpstreamCert {
		p.AddAddon(proxy.NewUpstreamCertAddon(false))
		log.Infoln("UpstreamCert config false")
	}

	p.AddAddon(&proxy.LogAddon{})
	p.AddAddon(web.NewWebAddon(config.WebAddr))

	if config.MapRemote != "" {
		mapRemote, err := addon.NewMapRemoteFromFile(config.MapRemote)
		if err != nil {
			log.Warnf("load map remote error: %v", err)
		} else {
			p.AddAddon(mapRemote)
		}
	}

	if config.MapLocal != "" {
		mapLocal, err := addon.NewMapLocalFromFile(config.MapLocal)
		if err != nil {
			log.Warnf("load map local error: %v", err)
		} else {
			p.AddAddon(mapLocal)
		}
	}

	if config.Dump != "" {
		dumper := addon.NewDumperWithFilename(config.Dump, config.DumpLevel)
		p.AddAddon(dumper)
	}

	log.Fatal(p.Start())
}

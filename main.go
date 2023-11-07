package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/stianeikeland/go-rpio/v4"
	pirelay "github.com/szampardi/pirelay/pkg"
)

var cfg *pirelay.Config

func init() {
	listenAddr := flag.String("l", ":8011", "listen address")
	timezone := flag.String("tz", "Europe/Rome", "timezone")
	latitude := flag.Float64("lat", 0, "latitude")
	longitude := flag.Float64("lon", 0, "longitude")
	configFile := flag.String("c", "", "json config file")
	dumpConfig := flag.Bool("cd", false, "dump config and exit")
	showVersion := flag.Bool("v", false, "print program version")
	flag.Parse()
	if *showVersion {
		fmt.Println(version())
		os.Exit(0)
	}
	cfg = pirelay.New(*timezone, *listenAddr, [2]float64{*latitude, *longitude})
	if *configFile != "" {
		fmt.Fprintf(os.Stderr, "loading configuration file %s\n", *configFile)
		f, err := os.Open(*configFile)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		if err = json.NewDecoder(f).Decode(cfg); err != nil {
			panic(err)
		}
	}
	if *dumpConfig {
		json.NewEncoder(os.Stdout).Encode(cfg)
		os.Exit(0)
	}
}

func main() {
	if err := rpio.Open(); err != nil {
		panic(err)
	}
	defer rpio.Close()
	var err error
	for _, r := range cfg.Relays {
		r, err = cfg.NewRelay(r.GPIO, r.Name, r.State, r.Sun, r.Schedules)
		if err != nil {
			panic(err)
		}
		if cfg.ListenAddr != "" {
			http.HandleFunc(fmt.Sprintf("/%d", r.GPIO), r.WebRelay)
			http.HandleFunc(fmt.Sprintf("/%s", r.Name), r.WebRelay)
			pirelay.Log(false, "added handlers for relay %s on GPIO %d", r.Name, r.GPIO)
		}
	}
	if cfg.ListenAddr != "" {
		http.HandleFunc("/", cfg.WebRoot)
		pirelay.Log(false, "server starting on %s", cfg.ListenAddr)
		if err := http.ListenAndServe(cfg.ListenAddr, nil); err != nil {
			panic(err)
		}
	} else {
		pirelay.Log(false, "pirelay started")
		select {}
	}
}

var (
	semver = "0.0.0-dev"
	commit = "git"
	built  = "a long time ago"
)

func version() string {
	return fmt.Sprintf("pirelay version %s commit %s built %s", semver, commit, built)
}

package main

import (
	"encoding/hex"
	"net"
	"os"
	"time"

	"github.com/metcalf/saypi/app"
	"github.com/metcalf/saypi/log"
	"github.com/namsral/flag"
	"github.com/zenazn/goji/bind"
	"github.com/zenazn/goji/graceful"
)

const (
	httpGrace = 5 * time.Second
)

type config struct {
	HTTPAddr string
}

func main() {
	appCfg, srvCfg, err := readConfiguration()
	if err != nil {
		log.Fatal("config_error", "Error parsing configuration",
			map[string]interface{}{"error": err})
	}

	a, err := app.New(appCfg)
	if err != nil {
		log.Fatal("init_error", "Error initializing app",
			map[string]interface{}{"error": err})
	}
	defer a.Close()

	listener, err := net.Listen("tcp", srvCfg.HTTPAddr)
	if err != nil {
		log.Fatal("listen_error", "Error attempting to listen on port",
			map[string]interface{}{"error": err, "address": srvCfg.HTTPAddr})
	}

	graceful.Timeout(httpGrace)
	graceful.HandleSignals()
	graceful.PreHook(func() {
		log.Print("app_stop", "Shutting down", nil)
	})
	log.Print("app_start", "Starting", map[string]interface{}{"address": listener.Addr()})
	bind.Ready()
	err = graceful.Serve(listener, a.Srv)
	if err != nil {
		log.Fatal("fatal_error", "Shutting down after a fatal error",
			map[string]interface{}{"error": err})
	}
}

func readConfiguration() (*app.Configuration, *config, error) {
	fl := flag.CommandLine
	var appCfg app.Configuration
	var srvCfg config

	fl.StringVar(&srvCfg.HTTPAddr, "http_addr", "0.0.0.0:8080", "Address to bind HTTP server")

	fl.StringVar(&appCfg.DBDSN, "db_dsn", "sslmode=disable dbname=saypi", "postgres data source name")
	fl.IntVar(&appCfg.DBMaxIdle, "db_max_idle", 2, "maximum number of idle DB connections")
	fl.IntVar(&appCfg.DBMaxOpen, "db_max_open", 100, "maximum number of open DB connections")

	fl.IntVar(&appCfg.IPPerMinute, "per_ip_rpm", 12, "maximum number of requests per IP per minute")
	fl.IntVar(&appCfg.IPRateBurst, "per_ip_burst", 5, "maximum instantaneous burst of requests per IP")

	userSecretStr := flag.String("user_secret", "", "hex encoded secret for generating secure user tokens")

	if err := fl.Parse(os.Args[1:]); err != nil {
		return nil, nil, err
	}

	userSecret, err := hex.DecodeString(*userSecretStr)
	if err != nil {
		return nil, nil, err
	}
	appCfg.UserSecret = userSecret

	return &appCfg, &srvCfg, nil
}

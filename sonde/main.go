// Command sonde e' l'applicazione unica: si collega al broker MQTT, salva le
// misure su SQLite e serve la dashboard web incorporata.
package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"sonde/internal/api"
	"sonde/internal/broker"
	"sonde/internal/hub"
	"sonde/internal/ingest"
	"sonde/internal/sim"
	"sonde/internal/store"
)

//go:embed web
var webAssets embed.FS

type config struct {
	broker    string
	topic     string
	clientID  string
	dbPath    string
	port      int
	dev       bool
	noBrowser bool
	noMQTT    bool

	test     bool
	testRoot string
	interval time.Duration
	count    int

	username string
	password string
}

func main() {
	cfg := parseFlags()

	logger := log.New(os.Stdout, "  ", log.LstdFlags)

	webFS, err := resolveWebFS(cfg.dev)
	if err != nil {
		logger.Fatalf("asset web: %v", err)
	}

	st, err := store.Open(cfg.dbPath)
	if err != nil {
		logger.Fatalf("database: %v", err)
	}
	defer st.Close()

	h := hub.New()
	server := api.New(st, h, webFS, logger)

	listener, err := listen(cfg.port)
	if err != nil {
		logger.Fatalf("listen: %v", err)
	}
	httpSrv := &http.Server{Handler: server.Handler()}

	var brk *broker.Broker
	if !cfg.noMQTT {
		ingestor := ingest.New(st, h, logger)
		brk, err = broker.Start(broker.Config{
			URL:      "tcp://" + cfg.broker,
			ClientID: cfg.clientID,
			Topic:    cfg.topic,
			Username: cfg.username,
			Password: cfg.password,
		}, ingestor.Handle)
		if err != nil {
			logger.Printf("MQTT non connesso (%v): la dashboard resta attiva", err)
		}
	}

	go func() {
		if err := httpSrv.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Printf("HTTP server: %v", err)
		}
	}()

	if cfg.test {
		opts := sim.Options{
			Broker:   cfg.broker,
			Root:     cfg.testRoot,
			Interval: cfg.interval,
			Count:    cfg.count,
		}
		go func() {
			if err := sim.Run(opts, func(f string, a ...any) { logger.Printf(f, a...) }); err != nil {
				logger.Printf("simulazione: %v", err)
			}
		}()
	}

	url := fmt.Sprintf("http://localhost:%d", cfg.port)
	printBanner(cfg, url)
	if !cfg.noBrowser {
		go func() {
			time.Sleep(400 * time.Millisecond)
			openBrowser(url)
		}()
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	fmt.Println("\n  Arresto in corso...")
	if brk != nil {
		brk.Stop()
	}
	_ = httpSrv.Close()
}

func parseFlags() config {
	port := 8080
	if p := os.Getenv("PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}

	var cfg config
	flag.StringVar(&cfg.broker, "broker", "broker.hivemq.com:1883", "indirizzo host:porta del broker MQTT")
	flag.StringVar(&cfg.topic, "topic", "421342415/#", "filtro topic MQTT da sottoscrivere")
	flag.StringVar(&cfg.clientID, "client-id", "", "client id MQTT (default casuale)")
	flag.StringVar(&cfg.dbPath, "db", "sonde.db", "percorso del database SQLite")
	flag.IntVar(&cfg.port, "port", port, "porta HTTP (env PORT)")
	flag.BoolVar(&cfg.dev, "dev", false, "serve la dashboard leggendo i file da disco")
	flag.BoolVar(&cfg.noBrowser, "no-browser", false, "non aprire il browser")
	flag.BoolVar(&cfg.noMQTT, "no-mqtt", false, "non collegarsi al broker MQTT")

	flag.BoolVar(&cfg.test, "test", false, "pubblica dati di prova da piu' dispositivi")
	flag.StringVar(&cfg.testRoot, "test-root", "421342415", "primo livello dei topic di test")
	flag.DurationVar(&cfg.interval, "interval", 2*time.Second, "intervallo tra i giri di test")
	flag.IntVar(&cfg.count, "count", 0, "numero di giri di test (0 = infinito)")

	flag.StringVar(&cfg.username, "username", "", "username MQTT")
	flag.StringVar(&cfg.password, "password", "", "password MQTT")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "sonde - monitoraggio sonde per i Vigili del Fuoco\n\n")
		fmt.Fprintf(os.Stderr, "Uso:\n  sonde [opzioni]\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if cfg.clientID == "" {
		cfg.clientID = fmt.Sprintf("sonde-%d", time.Now().UnixNano())
	}
	return cfg
}

func resolveWebFS(dev bool) (fs.FS, error) {
	if dev {
		return os.DirFS("web"), nil
	}
	return fs.Sub(webAssets, "web")
}

// listen prova la porta richiesta e, se occupata, le successive.
func listen(port int) (net.Listener, error) {
	var lastErr error
	for p := port; p < port+20; p++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", p))
		if err == nil {
			return ln, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func printBanner(cfg config, url string) {
	fmt.Println()
	fmt.Println("  ============================================================")
	fmt.Println("   sonde - monitoraggio sonde Vigili del Fuoco")
	fmt.Println("  ============================================================")
	fmt.Printf("   Dashboard : %s\n", url)
	fmt.Printf("   Broker    : %s   topic: %s\n", cfg.broker, cfg.topic)
	fmt.Printf("   Database  : %s\n", cfg.dbPath)
	if cfg.test {
		fmt.Printf("   Test      : attivo (root %s, intervallo %s)\n", cfg.testRoot, cfg.interval)
	}
	fmt.Println("   Ctrl+C per fermare.")
	fmt.Println()
}

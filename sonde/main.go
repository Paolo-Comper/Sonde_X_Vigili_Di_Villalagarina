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
	appconf "sonde/internal/config"
	"sonde/internal/hub"
	"sonde/internal/ingest"
	"sonde/internal/sim"
	"sonde/internal/store"
	"sonde/internal/topology"
)

//go:embed web
var webAssets embed.FS

//go:embed topology.json
var defaultTopology []byte

type config struct {
	broker    string
	topic     string
	clientID  string
	dbPath    string
	settings  string
	topology  string
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

	explicit map[string]bool
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

	topo, created, err := topology.LoadOrCreate(cfg.topology, defaultTopology)
	if err != nil {
		logger.Fatalf("topology: %v", err)
	}
	if created {
		logger.Printf("topology: creato %s dal default incorporato", cfg.topology)
	}

	mqttCfg := loadMQTTConfig(cfg, logger)

	h := hub.New()
	ingestor := ingest.New(st, h, logger)
	mqttMgr := broker.NewManager(ingestor.Handle, logger)
	if !cfg.noMQTT {
		if err := mqttMgr.Apply(brokerConfig(cfg, mqttCfg)); err != nil {
			logger.Printf("MQTT non connesso (%v): la dashboard resta attiva", err)
		}
	}

	server := api.New(st, h, topo, cfg.topology, cfg.settings, mqttCfg, mqttMgr, webFS, logger)

	listener, err := listen(cfg.port)
	if err != nil {
		logger.Fatalf("listen: %v", err)
	}
	httpSrv := &http.Server{Handler: server.Handler()}

	go func() {
		if err := httpSrv.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Printf("HTTP server: %v", err)
		}
	}()

	if cfg.test {
		if !cfg.explicit["test-root"] && mqttCfg.Group() != "" {
			cfg.testRoot = mqttCfg.Group()
		}
		opts := sim.Options{
			Broker:   mqttCfg.Broker,
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
	printBanner(cfg, url, mqttCfg)
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
	mqttMgr.Stop()
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
	flag.StringVar(&cfg.settings, "settings", "sonde-settings.json", "file delle impostazioni MQTT (credenziali incluse)")
	flag.StringVar(&cfg.topology, "topology", "topology.json", "file dei collegamenti tra sonde (env SONDE_TOPOLOGY)")
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
	cfg.explicit = make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { cfg.explicit[f.Name] = true })

	if cfg.clientID == "" {
		cfg.clientID = fmt.Sprintf("sonde-%d", time.Now().UnixNano())
	}
	if p := os.Getenv("SONDE_TOPOLOGY"); p != "" {
		cfg.topology = p
	}
	return cfg
}

// loadMQTTConfig combina flag, valori persistiti e default. I flag impostati
// esplicitamente hanno la precedenza sulla configurazione salvata.
func loadMQTTConfig(cfg config, logger *log.Logger) appconf.MQTT {
	base := appconf.MQTT{
		Broker:   cfg.broker,
		Prefix:   appconf.PrefixFromTopic(cfg.topic),
		Username: cfg.username,
		Password: cfg.password,
	}
	if saved, ok, err := appconf.Load(cfg.settings); err != nil {
		logger.Printf("impostazioni MQTT: %v", err)
	} else if ok {
		base = saved
	}

	if cfg.explicit["broker"] {
		base.Broker = cfg.broker
	}
	if cfg.explicit["topic"] {
		base.Prefix = appconf.PrefixFromTopic(cfg.topic)
	}
	if cfg.explicit["username"] {
		base.Username = cfg.username
	}
	if cfg.explicit["password"] {
		base.Password = cfg.password
	}
	return base.Normalized()
}

func brokerConfig(cfg config, mqttCfg appconf.MQTT) broker.Config {
	return broker.Config{
		URL:      mqttCfg.BrokerURL(),
		ClientID: cfg.clientID,
		Topic:    mqttCfg.Topic,
		Username: mqttCfg.Username,
		Password: mqttCfg.Password,
	}
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

func printBanner(cfg config, url string, mqttCfg appconf.MQTT) {
	fmt.Println()
	fmt.Println("  ============================================================")
	fmt.Println("   sonde - monitoraggio sonde Vigili del Fuoco")
	fmt.Println("  ============================================================")
	fmt.Printf("   Dashboard : %s\n", url)
	fmt.Printf("   Broker    : %s   topic: %s\n", mqttCfg.Broker, mqttCfg.Topic)
	fmt.Printf("   Database  : %s\n", cfg.dbPath)
	fmt.Printf("   Impostaz. : %s\n", cfg.settings)
	fmt.Printf("   Topologia : %s\n", cfg.topology)
	if cfg.test {
		fmt.Printf("   Test      : attivo (root %s, intervallo %s)\n", cfg.testRoot, cfg.interval)
	}
	fmt.Println("   Ctrl+C per fermare.")
	fmt.Println()
}

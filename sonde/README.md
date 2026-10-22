# sonde

Applicazione unica per il monitoraggio delle sonde dei Vigili del Fuoco di
Villalagarina. Un solo binario Go che:

- si collega a un broker **MQTT** e raccoglie i dati di **piu' dispositivi
  contemporaneamente**;
- salva le misure su un database **SQLite**;
- serve una **dashboard web** incorporata (nessun file esterno da copiare);
- disegna sulla mappa i **collegamenti di affluenza** tra le sonde (frecce) e
  ne mostra il **grafo**;
- esporta i dati in **CSV/JSON** con query SQL personalizzabile.

Il progetto nasce come evoluzione di
[mimosa-dashboard](https://github.com/Paolo-Comper/mimosa-dashboard), ma
gestisce piu' sonde insieme: ogni sonda misura sia il livello idrico sia la
qualita' dell'aria, senza distinzione di tipo.

## Requisiti

- Go 1.24 o superiore (per compilare)
- Un broker MQTT raggiungibile (default: `broker.hivemq.com:1883`)

## Compilazione

```bash
go build -o sonde .
```

Il binario e' **autosufficiente**: gli asset web sono inclusi con `go:embed`.

## Avvio

```bash
./sonde
```

All'avvio apre il browser su `http://localhost:8080`. Per la produzione senza
browser:

```bash
./sonde --no-browser
```

### Prova senza hardware

```bash
./sonde --test
```

La modalita' `--test` pubblica dati simulati da **4 sonde** (ognuna misura sia
il livello idrico sia la qualita' dell'aria) sullo stesso broker a cui l'app si
collega, utile per provare la dashboard end-to-end.

## Opzioni principali

| Flag | Default | Descrizione |
|------|---------|-------------|
| `--broker` | `broker.hivemq.com:1883` | Indirizzo `host:porta` del broker MQTT |
| `--topic` | `421342415/#` | Filtro MQTT sottoscritto |
| `--client-id` | casuale | Client id MQTT |
| `--username` / `--password` | vuoto | Credenziali MQTT |
| `--db` | `sonde.db` | Percorso del database SQLite |
| `--topology` | `topology.json` | File dei collegamenti tra sonde (env `SONDE_TOPOLOGY`) |
| `--port` | `8080` | Porta HTTP (sovrascrivibile con la variabile d'ambiente `PORT`) |
| `--dev` | off | Serve la dashboard leggendo i file da disco (utile in sviluppo) |
| `--no-browser` | off | Non apre il browser |
| `--no-mqtt` | off | Non si collega al broker (solo dashboard) |
| `--test` | off | Pubblica dati di prova da piu' dispositivi |
| `--test-root` | `421342415` | Primo livello dei topic di test |
| `--interval` | `2s` | Intervallo tra i giri della simulazione |
| `--count` | `0` (infinito) | Numero di giri della simulazione |

La porta si puo' impostare anche cosi':

```bash
PORT=9000 ./sonde
```

## Topic e payload unificato

Schema topic:

```
{root}/{device_id}[/data]
```

- `{root}` = `421342415`
- `{device_id}` = identificativo della sonda (es. `Rio_Molini_0`, `rpi-zero-1`)

Il **nome mostrato sul web e' il campo `label`** del payload; se manca, si usa
l'`id` ricavato dal topic.

L'app accetta due famiglie di payload e un dispositivo puo' inviarle anche
insieme:

**Sonda idrometrica** (JSON piatto):

```json
{
  "client-id": "id_TN_VL_0",
  "label": "Rio Molini 0",
  "lat": 45.915771,
  "lon": 11.024346,
  "ultrasonic_cm": 124.22,
  "tof_cm": 125.01,
  "rssi": 27,
  "net": "LTE"
}
```

**Stazione aria MIMOSA** (JSON annidato):

```json
{
  "label": "Aria Rovereto Centro",
  "pms": { "pm1": 12.42, "pm25": 15.97, "pm10": 21.5, "temperature": 14.24, "humidity": 66.85 },
  "dht22": { "temperature": 14.09, "humidity": 65.95 },
  "bme280": { "temperature": 14.34, "humidity": 65.45, "pressure": 101056.86 },
  "virtual": { "temperature": 14.22, "humidity": 65.7, "pressure": 101056.86 },
  "gps": { "lat": 45.89, "lon": 11.04, "alt": 180, "sats": 10, "valid": true }
}
```

I campi mancanti restano `NULL`: uno zero reale e' distinto da un campo assente.

## Database

SQLite, due tabelle:

- `devices` – anagrafica delle sonde viste (label, gruppo, tipo, posizione,
  primo/ultimo avvistamento);
- `measurements` – una riga per ogni misura, con una colonna per grandezza
  fisica (`water_level_cm`, `pm1`, `pm25`, `pm10`, `temp_*`, `hum_*`,
  `press_*`, `lat`, `lon`, `rssi`, `net`, ...) e la colonna booleana
  **`exported`**.

Lo schema e' volutamente "largo" per tenere in un'unica tabella sonde
idrometriche e stazioni aria, esportabile direttamente in CSV o interrogabile
con SQL.

## Dashboard

- riepilogo dispositivi / misure / esportate / in attesa;
- selezione del dispositivo (o "tutti"): non c'e' distinzione di tipo, ogni
  sonda mostra le grandezze che effettivamente invia;
- valori correnti e mappa delle posizioni; con "tutti" si vedono la mappa
  d'insieme con le **frecce delle affluenze** e il **grafo** della rete, con un
  dispositivo specifico compaiono i grafici storici e la mappa evidenzia la
  sonda selezionata;
- tabella delle ultime misure;
- aggiornamento live via **Server-Sent Events**;
- tema chiaro/scuro.

## Collegamenti di affluenza (`topology.json`)

La rete idrica si descrive con un file JSON (default `topology.json`). Il
default e' **incorporato nel binario**: all'avvio, se il file non esiste nella
cartella di lavoro, viene creato automaticamente con quel contenuto.

```json
{
  "links": [
    { "from": "Rio Molini 0", "to": "Rio Piazzo 0" },
    { "from": "Rio Val Morela 0", "to": "Rio Piazzo 0" }
  ]
}
```

- `from` / `to` devono essere la **label** o l'**id** di una sonda reale. I
  collegamenti verso punti senza sonda (es. lo sbocco in un fiume) vengono
  ignorati: niente frecce che puntano nel vuoto, e la sonda resta comunque nel
  grafo come nodo.
- Nella vista "tutti" la mappa disegna le frecce delle affluenze e compare il
  **grafo** della rete; con un dispositivo selezionato le frecce sono nascoste.
- Il pulsante **Modifica affluenze** (sopra mappa e grafo) apre un editor
  grafico: si scelgono sonda a monte e a valle, si aggiungono/rimuovono
  collegamenti e si salva. Il file viene riscritto in modo atomico e tutti i
  client collegati si aggiornano via SSE.

## Esportazione

Dal pulsante **Esporta** si puo':

- scrivere una **query SQL** (solo `SELECT`, validata lato server);
- vedere l'anteprima del risultato;
- scaricare in **CSV**, **JSON**, oppure **entrambi in un archivio ZIP**;
- segnare le righe esportate con il booleano `exported`, che diventa
  utilizzabile nella query (es. `SELECT * FROM measurements WHERE exported = 0`);
- cancellare le misure gia' esportate.

La query viene eseguita in una transazione read-only: un'istruzione di
scrittura non puo' modificare il database.

## API HTTP

| Metodo | Percorso | Descrizione |
|--------|----------|-------------|
| GET | `/api/devices` | Elenco dispositivi |
| GET | `/api/latest` | Ultima misura per dispositivo |
| GET | `/api/readings?device=&limit=` | Misure recenti |
| GET | `/api/stats` | Contatori aggregati |
| GET | `/api/topology` | Collegamenti di affluenza configurati |
| PUT | `/api/topology` | Salva i collegamenti (`{"links":[{"from":"A","to":"B"}]}`) |
| GET | `/api/schema` | Colonne della tabella `measurements` |
| GET | `/api/query?sql=&limit=` | Anteprima di una `SELECT` |
| GET | `/api/export?format=csv\|json\|zip&mark=0\|1&sql=` | Download |
| POST | `/api/clear-exported` | Elimina le misure esportate |
| GET | `/api/stream` | Stream SSE degli aggiornamenti |
| GET | `/healthz` | Health check |

## Test

```bash
go test ./...
```

I test coprono: parsing e normalizzazione dei payload, store SQLite ed export,
API HTTP e una catena completa **simulatore -> broker -> ingest -> SQLite**
mediante un broker MQTT in-process.

## Struttura

```
sonde/
  main.go                     avvio, flag e ciclo di vita
  topology.json               collegamenti di affluenza (default incorporato)
  integration_test.go         test end-to-end (broker in-process)
  internal/
    telemetry/                payload unificato + parsing topic
    store/                    SQLite (devices, measurements)
    export/                   CSV/JSON/ZIP + validazione SQL
    hub/                      fan-out SSE
    broker/                   client MQTT (paho)
    ingest/                   MQTT -> store -> SSE
    api/                      API HTTP e dashboard
    sim/                      simulatore --test
    topology/                 lettura dei collegamenti di affluenza
  web/                        dashboard (HTML/CSS/JS)
```

# Progetto Sonde per i Vigili del Fuoco – Villalagarina

Sistema di monitoraggio di piu' sonde (livello dei corsi d'acqua e qualita'
dell'aria) che inviano le misure via MQTT; un'unica applicazione le raccoglie,
le salva su SQLite e le mostra in una dashboard web.

## Struttura del repository

| Cartella | Contenuto |
|----------|-----------|
| [`sonde/`](sonde/) | L'applicazione completa (server MQTT + dashboard web) |
| [`Esp32Sim/`](Esp32Sim/) | Firmware/simulatore ESP32 che pubblica le misure |

### Applicazione (`sonde/`)

Un solo binario Go: si collega al broker MQTT, gestisce piu' dispositivi
contemporaneamente, salva su SQLite, serve la dashboard incorporata ed esporta
i dati in CSV/JSON con query SQL.

```bash
cd sonde
go build -o sonde .
./sonde --no-browser
```

Dettagli, opzioni e API: vedi [`sonde/README.md`](sonde/README.md).

### Firmware ESP32 (`Esp32Sim/`)

Progetto [PlatformIO](https://platformio.org/) per LILYGO T-SIM7600G-H con
sensori AJ-SR04M (ultrasuoni) e VL53L5CX (ToF). Pubblica su
`421342415/<device_id>` un JSON con `label`, posizione e misure.

```bash
cd Esp32Sim
pio run -t upload
```

## Requisiti

- Go 1.24+ (per l'applicazione)
- PlatformIO (per il firmware)
- Un broker MQTT raggiungibile (default: `broker.hivemq.com:1883`)

## Avvio rapido (senza hardware)

```bash
cd sonde
go run . --test
```

Apre la dashboard su `http://localhost:8080` con dati simulati da piu'
dispositivi.

## Note

- Il nome mostrato sul web e' il campo `label` del payload MQTT.
- La porta HTTP di default e' `8080`, modificabile con il flag `--port` o con
  la variabile d'ambiente `PORT`.

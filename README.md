# Progetto Sonde per i Vigili del Fuoco – Villalagarina

## Installation

### Dependencies

#### Required (runtime)

* Node.js
* npm
* Go (Golang)

#### Optional (development & testing)

* mosquitto-clients (for MQTT testing)
* platformio-cli (for firmware development)

## Install on Linux / Termux

### Debian / Ubuntu

```bash
sudo apt update
sudo apt install nodejs npm golang-go git

# Optional (testing)
sudo apt install mosquitto-clients
```

### Termux

```bash
pkg update
pkg install nodejs npm golang git

# Optional (testing)
pkg install mosquitto
```

## Clone the repository

```bash
git clone https://github.com/Paolo-Comper/Sonde_X_Vigili_Di_Villalagarina.git
cd Sonde_X_Vigili_Di_Villalagarina
```

## Run the project

Assuming you are inside the `Sonde_X_Vigili_Di_Villalagarina` directory.

### Terminal 1 – Server

By default, the server exposes a JSON file at: `http://localhost:8080/state.json`

You should see a message like: `Server HTTP su http://localhost:8080/state.json`

Run:

```bash
cd mqtt_state_server/
go run main.go
```

### Terminal 2 – Dashboard

```bash
cd Dashboard/
npx serve .
```

### Terminal 3 – Testing (optional)

```bash
./Test/test.sh
```

## Notes

* Make sure all dependencies are installed before running the project
* The dashboard reads data from the HTTP server
* MQTT testing tools are optional but useful during development


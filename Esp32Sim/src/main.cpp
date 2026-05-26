
/*
 * ============================================================
 *  MONITOR LIVELLO CORSO D'ACQUA  –  v2  (4G LTE)
 *  Hardware : LILYGO T-SIM7600G-H
 *  Sensori  : AJ-SR04M (ultrasuoni) + ST VL53L5CX (ToF laser)
 *  GPS      : integrato nel SIM7600G (AT+CGPS)  ← niente NEO-6M
 *  Rete     : LTE Cat-4 via SIM  →  MQTT
 *  Sleep    : deep-sleep tra misure
 *
 *  Librerie necessarie:
 *    - TinyGSM           (Volodymyr Shymanskyy)   ← stessa lib, modem diverso
 *    - PubSubClient      (Nick O'Leary)
 *    - SparkFun VL53L5CX (SparkFun Electronics)
 *    *** TinyGPS++ NON PIÙ NECESSARIA ***
 * ============================================================
 */

// #define TINY_GSM_MODEM_SIM7600
// #define TINY_GSM_DEBUG Serial

#include <Arduino.h>
#include <TinyGsmClient.h>
#include <PubSubClient.h>
#include <Wire.h>
#include <SparkFun_VL53L5CX_Library.h>

// ============================================================
//  MACRO
// ============================================================

#define DEBUG_MODE 1
#define DEBUG_SERIAL Serial

#if DEBUG_MODE
  #define DEBUG_PRINT(x)    DEBUG_SERIAL.print(x)
  #define DEBUG_PRINTLN(x)  DEBUG_SERIAL.println(x)
  #define DEBUG_PRINTF(...) DEBUG_SERIAL.printf(__VA_ARGS__)
#else
  #define DEBUG_PRINT(x)
  #define DEBUG_PRINTLN(x)
  #define DEBUG_PRINTF(...)
#endif

// ============================================================
//  CONFIGURAZIONE UTENTE
// ============================================================

static const char* APN        = "web";     // ← aggiornato
static const char* APN_USER   = "";
static const char* APN_PASS   = "";
static const char* SIM_PIN    = "1024";    // ← aggiunto (usa "" se SIM senza PIN)


static const char* MQTT_SERVER = "broker.hivemq.com";
static const int   MQTT_PORT   = 1883;
static const char* MQTT_TOPIC  = "421342415/Rio_Molini_0";
static const char* CLIENT_ID   = "id_TN_VL_0";
static const char* LABEL       = "Rio Molini 0";

static const uint32_t SLEEP_SEC   = 300;   // 5 minuti
static const uint32_t GPS_EVERY_N = 288;   // ogni 24 h

// ============================================================
//  PIN  –  T-SIM7600G-H
//
//  NOTA: esistono revisioni hardware diverse. Verifica sempre
//  il pinout stampato sul retro della tua scheda o su:
//  https://github.com/Xinyuan-LilyGO/LilyGO-T-A7670
// ============================================================

// SIM7600G (fissi sulla board)
#define MODEM_TX        27
#define MODEM_RX        26
#define MODEM_PWRKEY     4
#define MODEM_RST        5
#define MODEM_DTR       32   // Data Terminal Ready – NON usare per sensori
#define MODEM_RI        33   // Ring Indicator      – NON usare per sensori
#define MODEM_STATUS     3   // HIGH = modem acceso (lettura)

// AJ-SR04M – spostato su GPIO liberi (evita conflitto con DTR/RI)
#define US_TRIG         14
#define US_ECHO         13

// VL53L5CX – I2C standard (SDA=21, SCL=22, invariato)

// ============================================================
//  MEMORIA RTC
// ============================================================

RTC_DATA_ATTR uint32_t boot_count  = 0;
RTC_DATA_ATTR float    stored_lat  = 45.890f;
RTC_DATA_ATTR float    stored_lon  = 11.040f;
RTC_DATA_ATTR bool     gps_ever_ok = false;

// ============================================================
//  OGGETTI GLOBALI
// ============================================================

HardwareSerial SerialSIM(1);

TinyGsm        modem(SerialSIM);
TinyGsmClient  gsm_client(modem);
PubSubClient   mqtt_client(gsm_client);

SparkFun_VL53L5CX    vl53;
VL53L5CX_ResultsData vl53_data;

// ============================================================
//  ULTRASUONI AJ-SR04M
// ============================================================

float measure_ultrasonic()
{
    digitalWrite(US_TRIG, LOW);
    delayMicroseconds(2);
    digitalWrite(US_TRIG, HIGH);
    delayMicroseconds(10);
    digitalWrite(US_TRIG, LOW);
    long us = pulseIn(US_ECHO, HIGH, 35000UL);
    if (us == 0) return -1.0f;
    return (float)us * 0.01715f;
}

// ============================================================
//  ToF VL53L5CX
// ============================================================

float measure_vl53()
{
    uint32_t t0 = millis();
    while (!vl53.isDataReady() && millis() - t0 < 2000) delay(20);
    if (!vl53.isDataReady()) return -1.0f;
    if (!vl53.getRangingData(&vl53_data)) return -1.0f;

    float sum = 0; int cnt = 0;
    for (int i = 0; i < 64; i++) {
        uint16_t d = vl53_data.distance_mm[i];
        if (d > 20 && d < 4000) { sum += d; cnt++; }
    }
    return cnt > 0 ? sum / cnt / 10.0f : -1.0f;
}

// ============================================================
//  GPS INTEGRATO SIM7600G  (via AT commands, nessun hardware extra)
// ============================================================

/**
 * Abilita il GPS interno del SIM7600, aspetta una fix,
 * poi disabilita il GPS per risparmiare energia.
 *
 * TinyGSM invia internamente:
 *   AT+CGPS=1       per abilitare
 *   AT+CGPSINFO     per leggere la posizione
 *   AT+CGPS=0       per disabilitare
 */
bool read_gps_internal(float& lat, float& lon, uint32_t timeout_ms = 120000UL)
{
    DEBUG_PRINTLN(F("[GPS] Abilitazione GPS interno SIM7600..."));
    if (!modem.enableGPS()) {
        DEBUG_PRINTLN(F("[GPS] enableGPS() fallito"));
        return false;
    }

    float speed, alt, accuracy;
    int   vsat, usat;
    uint32_t t0 = millis();

    while (millis() - t0 < timeout_ms)
    {
        if (modem.getGPS(&lat, &lon, &speed, &alt, &vsat, &usat, &accuracy))
        {
            // Fix valida: disabilita GPS e restituisci posizione
            DEBUG_PRINTF("[GPS] Fix OK  lat=%.6f lon=%.6f  sat=%d  acc=%.1f m\n",
                          lat, lon, usat, accuracy);
            modem.disableGPS();
            return true;
        }
        DEBUG_PRINTF("[GPS] In attesa fix... sat_view=%d sat_use=%d\n", vsat, usat);
        delay(3000);
    }

    modem.disableGPS();
    DEBUG_PRINTLN(F("[GPS] Timeout fix GPS"));
    return false;
}

bool read_sim_response(char* out, size_t out_size, uint32_t timeout_ms)
{
    size_t idx = 0;
    uint32_t start = millis();

    while (millis() - start < timeout_ms && idx < out_size - 1)
    {
        while (SerialSIM.available() && idx < out_size - 1)
        {
            out[idx++] = (char)SerialSIM.read();
        }
        delay(5);
    }

    out[idx] = '\0';
    return idx > 0;
}

bool read_gps_internal_2(float& lat_out, float& lon_out)
{
    DEBUG_PRINTLN(F("[GPS] Avvio GNSS"));

    SerialSIM.println("AT+CGPS=1");
    delay(1500);

    char resp[160];
    char data[120];

    uint32_t start = millis();
    bool printed_searching = false;

    while (millis() - start < 120000UL)
    {
        SerialSIM.println("AT+CGPSINFO");

        if (!read_sim_response(resp, sizeof(resp), 800))
        {
            delay(1000);
            continue;
        }

        char* ptr = strstr(resp, "+CGPSINFO:");

        if (!ptr)
        {
            if (!printed_searching)
            {
                DEBUG_PRINTLN(F("[GPS] searching fix..."));
                printed_searching = true;
            }
            delay(1500);
            continue;
        }

        ptr += 10;

        while (*ptr == ' ' || *ptr == ':')
            ptr++;

        strncpy(data, ptr, sizeof(data) - 1);
        data[sizeof(data) - 1] = '\0';

        if (strstr(data, ",,,,,,,"))
        {
            if (!printed_searching)
            {
                DEBUG_PRINTLN(F("[GPS] searching fix..."));
                printed_searching = true;
            }
            delay(1500);
            continue;
        }

        float lat_raw, lon_raw;
        char ns, ew;

        if (sscanf(data, "%f,%c,%f,%c",
                   &lat_raw, &ns,
                   &lon_raw, &ew) != 4)
        {
            DEBUG_PRINTLN(F("[GPS] parse error"));
            delay(1000);
            continue;
        }

        int lat_deg = (int)(lat_raw / 100.0);
        float lat_min = fmodf(lat_raw, 100.0f);

        int lon_deg = (int)(lon_raw / 100.0);
        float lon_min = fmodf(lon_raw, 100.0f);

        lat_out = lat_deg + (lat_min / 60.0f);
        lon_out = lon_deg + (lon_min / 60.0f);

        if (ns == 'S') lat_out = -lat_out;
        if (ew == 'W') lon_out = -lon_out;

        DEBUG_PRINTLN(F("[GPS] FIX ACQUIRED"));
        DEBUG_PRINTF("[GPS] lat=%.6f lon=%.6f\n", lat_out, lon_out);

        SerialSIM.println("AT+CGPS=0");

        return true;
    }

    DEBUG_PRINTLN(F("[GPS] TIMEOUT NO FIX"));
    SerialSIM.println("AT+CGPS=0");
    return false;
}

// ============================================================
//  MODEM SIM7600G-H
// ============================================================

/**
 * Sequenza di accensione per SIM7600G-H.
 * Differisce dal SIM800L: nessun POWER_ON separato,
 * PWRKEY ha un impulso LOW->HIGH->LOW di circa 1 secondo.
 */
void modem_power_on()
{
    pinMode(MODEM_PWRKEY, OUTPUT);
    digitalWrite(MODEM_PWRKEY, LOW);
    delay(100);
    digitalWrite(MODEM_PWRKEY, HIGH);
    delay(1000);                        // impulso 1 s
    digitalWrite(MODEM_PWRKEY, LOW);

    // Imposta DTR basso (necessario per tenere sveglio il modem)
    pinMode(MODEM_DTR, OUTPUT);
    digitalWrite(MODEM_DTR, LOW);

    delay(3500);   // attendi boot completo del SIM7600
}

void modem_power_off()
{
    // Su SIM7600 un impulso lungo su PWRKEY spegne il modem
    digitalWrite(MODEM_PWRKEY, HIGH);
    delay(2500);
    digitalWrite(MODEM_PWRKEY, LOW);
}

bool gsm_connect()
{
    DEBUG_PRINTLN(F("[GSM] Accensione SIM7600G-H..."));
    modem_power_on();

    SerialSIM.begin(115200, SERIAL_8N1, MODEM_RX, MODEM_TX);

    // Il SIM7600 usa 115200 (vs 9600 del SIM800L)
    if (!modem.restart()) {
        DEBUG_PRINTLN(F("[GSM] Restart fallito"));
        return false;
    }

    // DEBUG_PRINT(F("[GSM] Modem: "));
    // DEBUG_PRINTLN(modem.getModemInfo());

    // ── SIM PIN ──────────────────────────────────────────────
    // Controlla lo stato della SIM prima di sbloccarla:
    // evita di inviare il PIN se la SIM è già sbloccata
    // (dopo un restart caldo il PIN potrebbe non essere richiesto)
    SimStatus sim_status = modem.getSimStatus();
    DEBUG_PRINT(F("[GSM] Stato SIM: "));
    switch (sim_status) {
        case SIM_READY:
            DEBUG_PRINTLN(F("pronta (nessun PIN richiesto)"));
            break;
        case SIM_LOCKED:
            DEBUG_PRINT(F("PIN richiesto – sblocco..."));
            if (!modem.simUnlock(SIM_PIN)) {
                DEBUG_PRINTLN(F(" FALLITO (PIN errato o SIM bloccata)"));
                return false;
            }
            DEBUG_PRINTLN(F(" OK"));
            delay(2000);   // attendi che la SIM si registri dopo lo sblocco
            break;
        case SIM_ANTITHEFT_LOCKED:
            DEBUG_PRINTLN(F("ERRORE: SIM bloccata da anti-furto (PUK richiesto)"));
            return false;
        default:
            DEBUG_PRINTLN(F("ERRORE: SIM non rilevata o guasta"));
            return false;
    }
    // ─────────────────────────────────────────────────────────

    String imei = modem.getIMEI();
    DEBUG_PRINT(F("[GSM] IMEI: "));
    DEBUG_PRINTLN(imei);

    {
        DEBUG_PRINT(F("[GSM] Attesa rete LTE..."));
        int error = modem.waitForNetwork(90000UL);

        if (!error) {       // SIM7600 può impiegare più tempo
            DEBUG_PRINT(F(" FALLITO con errore "));
            DEBUG_PRINTLN(error);
            return false;
        }
        DEBUG_PRINT(F(" OK | RSSI="));
        DEBUG_PRINT(modem.getSignalQuality());
    }

    // Mostra tipo di rete (LTE, WCDMA, ecc.)
    String nettype = modem.getNetworkMode() == 38 ? "LTE" : "2G/3G";
    DEBUG_PRINT(F(" | Rete="));
    DEBUG_PRINTLN(nettype);

    DEBUG_PRINT(F("[GSM] GPRS (APN="));
    DEBUG_PRINT(APN);
    DEBUG_PRINT(F(")..."));
    if (!modem.gprsConnect(APN, APN_USER, APN_PASS)) {
        DEBUG_PRINTLN(F(" FALLITO"));
        return false;
    }
    DEBUG_PRINT(F(" IP: "));
    DEBUG_PRINTLN(modem.localIP());
    return true;
}

void gsm_disconnect()
{
    modem.gprsDisconnect();
    delay(200);
    modem_power_off();
}

// ============================================================
//  MQTT
// ============================================================

bool mqtt_connect()
{
    mqtt_client.setServer(MQTT_SERVER, MQTT_PORT);
    for (int i = 0; i < 4; i++) {
        DEBUG_PRINT(F("[MQTT] Connessione..."));
        if (mqtt_client.connect(CLIENT_ID)) {
            DEBUG_PRINTLN(F(" OK"));
            return true;
        }
        DEBUG_PRINTF(" stato=%d\n", mqtt_client.state());
        delay(3000);
    }
    return false;
}

/*
 * Formato JSON:
 * {
 *   "client-id"     : "esp32_1",
 *   "label"         : "Adige",
 *   "lat"           : 45.890000,
 *   "lon"           : 11.040000,
 *   "ultrasonic_cm" : 123.45,
 *   "tof_cm"        : 122.80,
 *   "rssi"          : 25,
 *   "net"           : "LTE"
 * }
 */
void send_data(float us_cm, float tof_cm, float lat, float lon)
{
    int rssi = modem.getSignalQuality();
    String net = modem.getNetworkMode() == 38 ? "LTE" : "2G/3G";

    char buf[350];
    int n = snprintf(buf, sizeof(buf),
        "{\"client-id\":\"%s\",\"label\":\"%s\","
        "\"lat\":%.6f,\"lon\":%.6f,"
        "\"ultrasonic_cm\":%.2f,\"tof_cm\":%.2f,"
        "\"rssi\":%d,\"net\":\"%s\"}",
        CLIENT_ID, LABEL, lat, lon, us_cm, tof_cm, rssi, net.c_str());

    if (n > 0 && n < (int)sizeof(buf)) {
        bool ok = mqtt_client.publish(MQTT_TOPIC, buf, (unsigned int)n);
        DEBUG_PRINTLN(ok ? F("[MQTT] Inviato OK") : F("[MQTT] ERRORE invio"));
        DEBUG_PRINTLN(buf);
    }
}

// ============================================================
//  SETUP
// ============================================================

void setup()
{
    Serial.begin(115200);
    boot_count++;
    DEBUG_PRINTF("\n=== Boot #%lu ===\n", (unsigned long)boot_count);

    // 1. Ultrasuoni (GPIO14/13, lontani da DTR/RI del modem)
    pinMode(US_TRIG, OUTPUT);
    pinMode(US_ECHO, INPUT);
    digitalWrite(US_TRIG, LOW);

    // 2. VL53L5CX
    Wire.begin();
    bool vl53_ok = vl53.begin();
    if (vl53_ok) {
        vl53.setResolution(64);
        vl53.startRanging();
        DEBUG_PRINTLN(F("[VL53] OK"));
    } else {
        DEBUG_PRINTLN(F("[VL53] Non trovato! Controlla I2C."));
    }

    // 3. Misure sensori (prima di accendere il modem, per risparmiare energia)
    float us_cm  = measure_ultrasonic();
    float tof_cm = vl53_ok ? measure_vl53() : -1.0f;
    if (vl53_ok) vl53.stopRanging();
    DEBUG_PRINTF("[SENS] US=%.2f cm | ToF=%.2f cm\n", us_cm, tof_cm);

    // 4. Connessione GSM (accende il modem una sola volta per tutto il ciclo)
    if (gsm_connect())
    {
        // 5. GPS ogni GPS_EVERY_N cicli – usa il GPS integrato del SIM7600
        if (boot_count % GPS_EVERY_N == 1) {
            DEBUG_PRINTLN(F("[GPS] Tentativo fix GPS interno (max 120 s)..."));
            float lat, lon;
            if (read_gps_internal_2(lat, lon)) {
                stored_lat = lat;
                stored_lon = lon;
                gps_ever_ok = true;
            } else {
                DEBUG_PRINTLN(F("[GPS] Timeout, uso posizione precedente."));
            }
        }

        // 6. Invio MQTT
        if (mqtt_connect()) {
            send_data(us_cm, tof_cm, stored_lat, stored_lon);
            mqtt_client.disconnect();
        }
        gsm_disconnect();
    }

    // 7. Deep sleep
    uint32_t next_gps = GPS_EVERY_N - (boot_count % GPS_EVERY_N);
    DEBUG_PRINTF("[SLEEP] %lu s | GPS tra %lu cicli\n",
                  (unsigned long)SLEEP_SEC, (unsigned long)next_gps);
    Serial.flush();
    esp_sleep_enable_timer_wakeup((uint64_t)SLEEP_SEC * 1000000ULL);
    esp_deep_sleep_start();
}

void loop() {}


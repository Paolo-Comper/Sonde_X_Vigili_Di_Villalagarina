#include "Arduino.h"
#include <WiFi.h>
#include <PubSubClient.h>

static const char* wifi_ssid  = "Paolo";
static const char* wifi_pass  = "12345678";

static const char* mqtt_server = "broker.hivemq.com";
static const int   mqtt_port   = 1883;

static const char* mqtt_topic  = "Sonde_X_Vigili_Di_Villalagarina/fiume1";
static const char* client_id = "esp32_1";
static const char* label = "Adige";

WiFiClient wifi_client;
PubSubClient mqtt_client(wifi_client);

float water_level = 2.9f;

void wifi_connect()
{
    WiFi.mode(WIFI_STA);
    WiFi.begin(wifi_ssid, wifi_pass);

    while (WiFi.status() != WL_CONNECTED)
    {
        delay(500);
        Serial.print(".");
    }

    Serial.println("\nWiFi connected");
}

void mqtt_reconnect()
{
    mqtt_client.setServer(mqtt_server, mqtt_port);

    while (!mqtt_client.connected())
    {
        Serial.print("Attempting MQTT connection...");

        if (mqtt_client.connect(client_id))
        {
            Serial.println("connected");
        }
        else
        {
            Serial.print("failed, state=");
            Serial.println(mqtt_client.state());
            delay(2000);
        }
    }
}

void send_sensor()
{
    char buffer[256];

    int written = snprintf(
        buffer,
        sizeof(buffer),
        R"({ "client-id": "%s", "label": "%s", "lat": 45.890, "lon": 11.040, "value": %.4f })",
        client_id, label, water_level
    );

    if (written > 0 && written < (int)sizeof(buffer))
    {
        mqtt_client.publish(mqtt_topic, buffer, written);
    }
    else
    {
        Serial.println("Errore costruzione JSON");
    }
}

void setup()
{
    Serial.begin(115200);

    wifi_connect();
    mqtt_client.setServer(mqtt_server, mqtt_port);
}

void loop()
{
    if (WiFi.status() != WL_CONNECTED)
    {
        wifi_connect();
    }

    if (!mqtt_client.connected())
    {
        mqtt_reconnect();
    }

    mqtt_client.loop();

    water_level = random(0, 10);
    send_sensor();

    delay(5000);
}


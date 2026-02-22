#include "Arduino.h"
#include <WiFi.h>
#include <PubSubClient.h>
#include <ArduinoJson.h>

static const char* wifi_ssid  = "Paolo";
static const char* wifi_pass  = "12345678";

static const char* mqtt_server = "test.mosquitto.org";
static const int   mqtt_port   = 1883;

static const char* mqtt_topic  = "sensors/data";

WiFiClient wifi_client;
PubSubClient mqtt_client(wifi_client);

float water_level = 2.9;

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
    // setServer prima di tentare di connettere
    mqtt_client.setServer(mqtt_server, mqtt_port);

    while (!mqtt_client.connected())
    {
        Serial.print("Attempting MQTT connection...");
        // usa la firma base con solo clientID
        if (mqtt_client.connect("esp32_fiume1"))
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
    StaticJsonDocument<256> doc;

    doc["id"]  = "fiume1";
    doc["value"] = water_level;
    doc["lat"] = 45.890;
    doc["lon"] = 11.040;

    char buffer[256];
    size_t n = serializeJson(doc, buffer);

    // pubblica con lunghezza precisa
    mqtt_client.publish(mqtt_topic, buffer, n);
}

void setup()
{
    Serial.begin(115200);

    wifi_connect();

    // imposta server MQTT qui oppure nel reconnect
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

    send_sensor();

    delay(5000);
}

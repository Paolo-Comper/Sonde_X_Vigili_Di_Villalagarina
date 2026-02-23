#include "Arduino.h"
#include <WiFi.h>
#include <PubSubClient.h>

static const char* wifi_ssid  = "Paolo";
static const char* wifi_pass  = "12345678";

static const char* mqtt_server = "test.mosquitto.org";
static const int   mqtt_port   = 1883;

static const char* mqtt_topic  = "Sonde_X_Vigili_Di_Villalagarina/fiume1";

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
    char buffer[256];

    int written = snprintf(
        buffer,
        sizeof(buffer),
        R"({ "id": "fiume1", "value": %.4f, "lat": 45.890, "lon": 11.040 })",
        water_level
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

    send_sensor();

    delay(5000);
}

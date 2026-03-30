#!/usr/bin/env bash

BROKER="broker.hivemq.com"
BASE_TOPIC="Sonde_X_Vigili_Di_Villalagarina"

while true; do
  for i in 1 2 3 4 5; do
    client_id="esp32_$i"
    label="Adige_$i"

    lat=$(echo "45.890 + 0.001*$i" | bc)
    lon=$(echo "11.040 + 0.001*$i" | bc)
    value=$(shuf -i 0-10 -n 1)

    payload=$(printf '{ "client-id": "%s", "label": "%s", "lat": %.3f, "lon": %.3f, "value": %.4f }' \
      "$client_id" "$label" "$lat" "$lon" "$value")

    mosquitto_pub -h "$BROKER" \
                  -t "$BASE_TOPIC/fiume$i" \
                  -m "$payload"

    sleep 0.1
  done

  sleep 5
done


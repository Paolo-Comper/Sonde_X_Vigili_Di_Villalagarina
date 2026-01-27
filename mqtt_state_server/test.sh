while true; do
  for i in 1 2 3 4 5; do
    mosquitto_pub -h test.mosquitto.org -t Sonde_X_Vigili_Di_Villalagarina/fiume$i/value -m "$(shuf -i 1-10 -n 1)"
    mosquitto_pub -h test.mosquitto.org -t Sonde_X_Vigili_Di_Villalagarina/fiume$i/lat   -m "$(echo "45.8 + 0.01*$i" | bc)"
    mosquitto_pub -h test.mosquitto.org -t Sonde_X_Vigili_Di_Villalagarina/fiume$i/lon   -m "$(echo "11.0 + 0.01*$i" | bc)"
  done
  sleep 5
done

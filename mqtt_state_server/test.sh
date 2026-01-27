while true; do
  mosquitto_pub -h test.mosquitto.org -t Sonde_X_Vigili_Di_Villalagarina/fiume1/value -m "$(shuf -i 1-10 -n 1)"
  mosquitto_pub -h test.mosquitto.org -t Sonde_X_Vigili_Di_Villalagarina/fiume1/lat   -m "45.89"
  mosquitto_pub -h test.mosquitto.org -t Sonde_X_Vigili_Di_Villalagarina/fiume1/lon   -m "11.04"
  sleep 5
done


# sudo apt install mosquitto
# sudo systemctl start mosquitto

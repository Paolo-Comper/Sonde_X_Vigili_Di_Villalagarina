/* Stato globale della dashboard. */

const ALL = '__all__';
const MAX_POINTS = 500;
const READINGS_LIMIT = 500;

const DEFAULT_SQL = 'SELECT * FROM measurements WHERE exported = 0 ORDER BY ts DESC LIMIT 1000';

let devices = [];
let readings = [];
let latestByDevice = {};
let stats = {};
let topology = { links: [] };
let mqttConfig = { broker: '', prefix: '', topic: '', username: '', password_set: false };
let mqttStatus = { connected: false };
let selectedDevice = ALL;
let sse = null;
let renderTimer = null;

function visibleDevices() {
  return devices;
}

function deviceById(id) {
  return devices.find(d => d.id === id) || null;
}

function selectedDeviceLabel() {
  if (selectedDevice === ALL) return 'Tutti i dispositivi';
  const d = deviceById(selectedDevice);
  return d ? d.label : selectedDevice;
}

/* Misure visibili in tabella per il dispositivo selezionato. */
function visibleReadings() {
  return readings.filter(r => selectedDevice === ALL || r.device_id === selectedDevice);
}

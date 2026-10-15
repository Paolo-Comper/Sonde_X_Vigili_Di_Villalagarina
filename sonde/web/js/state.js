/* Stato globale della dashboard. */

const ALL = '__all__';
const MAX_POINTS = 500;
const READINGS_LIMIT = 500;

const DEFAULT_SQL = 'SELECT * FROM measurements WHERE exported = 0 ORDER BY ts DESC LIMIT 1000';

let devices = [];
let readings = [];
let latestByDevice = {};
let stats = {};
let selectedDevice = ALL;
let selectedKind = 'all';
let sse = null;
let renderTimer = null;

function kindMatches(kind) {
  if (selectedKind === 'all') return true;
  if (kind === selectedKind) return true;
  return kind === 'misto';
}

function visibleDevices() {
  return devices.filter(d => kindMatches(d.kind));
}

function deviceById(id) {
  return devices.find(d => d.id === id) || null;
}

function selectedDeviceLabel() {
  if (selectedDevice === ALL) return 'Tutti i dispositivi';
  const d = deviceById(selectedDevice);
  return d ? d.label : selectedDevice;
}

/* Misure visibili in tabella secondo i filtri correnti. */
function visibleReadings() {
  return readings.filter(r =>
    kindMatches(r.kind) &&
    (selectedDevice === ALL || r.device_id === selectedDevice)
  );
}

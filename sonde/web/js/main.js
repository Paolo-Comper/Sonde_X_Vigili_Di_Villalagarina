/* Inizializzazione e rendering della dashboard. */

function renderStats() {
  $('statDevices').textContent = stats.devices !== undefined ? stats.devices : devices.length;
  $('statReadings').textContent = stats.readings !== undefined ? stats.readings : 0;
  $('statExported').textContent = stats.exported !== undefined ? stats.exported : 0;
  $('statPending').textContent = stats.pending !== undefined ? stats.pending : 0;
  $('statLast').textContent = stats.last_ts ? fmtTime(stats.last_ts) : DASH;
}

function renderDeviceSelect() {
  const sel = $('deviceSelect');
  if (!sel) return;
  const list = visibleDevices();
  let html = '<option value="' + ALL + '">Tutti i dispositivi</option>';
  list.forEach(d => {
    html += '<option value="' + escapeHtml(d.id) + '">' + escapeHtml(d.label) + '</option>';
  });
  sel.innerHTML = html;
  const keep = selectedDevice !== ALL && list.some(d => d.id === selectedDevice);
  sel.value = keep ? selectedDevice : ALL;
  selectedDevice = sel.value;
}

function render() {
  renderStats();
  renderDeviceSelect();
  renderCurrentValues();
  updateCharts();
  renderTable();
  updateMap();
  updateGraph();
}

async function refreshAll() {
  await Promise.all([
    loadDevices(), loadStats(), loadReadings(), loadLatest(), loadTopology(), loadSettings()
  ]);
  render();
}

async function onDeviceChange() {
  selectedDevice = $('deviceSelect').value;
  await loadReadings();
  render();
}

function init() {
  initCharts();
  initMap();
  initExportUI();

  $('deviceSelect').addEventListener('change', onDeviceChange);

  refreshAll()
    .then(connectSSE)
    .catch(err => {
      console.error(err);
      setStatus('disconnected', 'Errore di connessione al server');
    });

  setInterval(() => {
    loadStats().then(renderStats).catch(() => {});
  }, 15000);
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', init);
} else {
  init();
}

/* Accesso alle API del server e stream SSE. */

async function fetchJSON(url) {
  const r = await fetch(url);
  if (!r.ok) {
    let message = r.statusText;
    try { message = (await r.json()).error || message; } catch (e) {}
    throw new Error(message);
  }
  return r.json();
}

async function loadDevices() {
  const out = await fetchJSON('/api/devices');
  devices = (out.devices || []).slice().sort((a, b) =>
    String(a.label).localeCompare(String(b.label), 'it'));

  if (selectedDevice !== ALL && !deviceById(selectedDevice)) {
    selectedDevice = ALL;
  }
}

async function loadStats() {
  stats = await fetchJSON('/api/stats');
}

async function loadReadings() {
  const params = new URLSearchParams({ limit: READINGS_LIMIT });
  if (selectedDevice !== ALL) params.set('device', selectedDevice);
  const out = await fetchJSON('/api/readings?' + params.toString());
  readings = out.data || [];
}

async function loadLatest() {
  const out = await fetchJSON('/api/latest');
  latestByDevice = {};
  (out.data || []).forEach(m => { latestByDevice[m.device_id] = m; });
}

function setStatus(state, text) {
  const el = $('connectionStatus');
  if (!el) return;
  el.className = 'status-indicator ' + (state === 'connected' ? 'connected' : 'disconnected');
  el.textContent = text || (state === 'connected' ? 'Connesso' : 'Disconnesso');
}

function connectSSE() {
  if (sse) sse.close();
  setStatus('connecting', 'Connessione...');
  sse = new EventSource('/api/stream');

  sse.addEventListener('hello', () => setStatus('connected'));
  sse.onopen = () => setStatus('connected');
  sse.onerror = () => setStatus('disconnected', 'Riconnessione...');
  sse.addEventListener('reading', e => {
    try { onLiveReading(JSON.parse(e.data)); } catch (err) { console.warn(err); }
  });
  sse.addEventListener('exported', () => {
    loadStats().then(scheduleRender).catch(() => {});
  });
}

function onLiveReading(m) {
  if (!m || !m.device_id) return;
  latestByDevice[m.device_id] = m;

  if (selectedDevice === ALL || m.device_id === selectedDevice) {
    readings.push(m);
    if (readings.length > MAX_POINTS) readings = readings.slice(-MAX_POINTS);
  }

  if (!deviceById(m.device_id)) {
    loadDevices().then(scheduleRender).catch(() => {});
  }
  scheduleRender();
}

function scheduleRender() {
  if (renderTimer) return;
  renderTimer = setTimeout(() => {
    renderTimer = null;
    render();
  }, 400);
}

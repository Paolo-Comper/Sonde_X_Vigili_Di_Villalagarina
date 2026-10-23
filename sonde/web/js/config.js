/* Configurazione MQTT (prefisso, broker) e aggiunta dispositivi. */

function topicFromPrefix(prefix) {
  const p = String(prefix || '').trim();
  if (!p) return '#';
  if (p.includes('+') || p.includes('#')) return p;
  return p.replace(/\/+$/, '') + '/#';
}

function currentPrefix() {
  const el = $('topicPrefixInput');
  const p = (el ? el.value : (mqttConfig.prefix || '')).trim();
  return p.replace(/\/+$/, '');
}

function currentSensorId() {
  return selectedDevice && selectedDevice !== ALL ? selectedDevice : '';
}

/* Topic del sensore selezionato (o filtro dell'osservatorio per "Tutti i sensori"). */
function currentTopicForDisplay() {
  const prefix = currentPrefix();
  if (!prefix) return '#';
  if (prefix.includes('+') || prefix.includes('#')) return topicFromPrefix(prefix);
  const sensor = currentSensorId();
  return sensor ? prefix + '/' + sensor : topicFromPrefix(prefix);
}

function updateTopicDisplay() {
  const el = $('currentTopic');
  if (!el) return;
  const topic = currentTopicForDisplay();
  el.textContent = topic;
  el.title = currentSensorId()
    ? 'Topic del sensore selezionato. E\' accettato anche ' + topic + '/data. La dashboard riceve tutti i sensori sotto l\'osservatorio.'
    : 'La dashboard ascolta tutti i sensori sotto l\'osservatorio.';
}

function renderDeviceTopicPreview() {
  const el = $('deviceTopicPreview');
  if (!el) return;
  const idEl = $('deviceId');
  const id = idEl && idEl.value.trim() ? idEl.value.trim() : '{id}';
  el.textContent = (currentPrefix() || '{prefisso}') + '/' + id;
}

function renderMQTTStatus() {
  const el = $('mqttStatus');
  if (!el) return;
  const st = mqttStatus || {};
  el.className = 'status-indicator ' + (st.connected ? 'connected' : 'disconnected');
  el.textContent = st.connected ? 'Connesso' : (st.error ? 'Errore' : 'Disconnesso');
  el.title = st.error
    ? 'MQTT: ' + st.error
    : 'Connessione del server al broker MQTT.';
}

function renderMQTT() {
  const prefixEl = $('topicPrefixInput');
  if (prefixEl && document.activeElement !== prefixEl) prefixEl.value = mqttConfig.prefix || '';
  const brokerEl = $('brokerInput');
  if (brokerEl && document.activeElement !== brokerEl) brokerEl.value = mqttConfig.broker || '';
  updateTopicDisplay();
  renderMQTTStatus();
  renderDeviceTopicPreview();
}

async function loadSettings() {
  try {
    const out = await fetchJSON('/api/settings');
    mqttConfig = out.mqtt || {};
    mqttStatus = out.status || {};
    renderMQTT();
  } catch (err) {
    console.warn('Impostazioni MQTT non disponibili:', err);
  }
}

function configMessage(text, ok) {
  const el = $('configMessage');
  if (!el) return;
  el.className = 'export-message' + (text ? (ok ? ' ok' : ' error') : '');
  el.textContent = text || '';
}

async function applySettings() {
  const button = $('mqttApply');
  const prefix = $('topicPrefixInput').value.trim();
  if (!prefix) {
    configMessage('Inserisci un prefisso topic.', false);
    return;
  }

  const body = {
    broker: $('brokerInput').value.trim(),
    prefix: prefix
  };

  button.disabled = true;
  configMessage('Connessione al broker...', true);
  try {
    const res = await fetch('/api/settings', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
    const data = await res.json();
    mqttConfig = data.mqtt || mqttConfig;
    mqttStatus = data.status || {};
    renderMQTT();
    if (!res.ok) {
      throw new Error((data.status && data.status.error) || data.error || res.statusText);
    }
    configMessage('Impostazioni salvate. Topic: ' + (mqttConfig.topic || ''), true);
    loadDevices().then(scheduleRender).catch(() => {});
  } catch (err) {
    configMessage(err.message, false);
    renderMQTTStatus();
  } finally {
    button.disabled = false;
  }
}

function openDevice() {
  const overlay = $('deviceOverlay');
  if (!overlay) return;
  $('deviceId').value = '';
  $('deviceLabel').value = '';
  $('deviceLat').value = '';
  $('deviceLon').value = '';
  deviceMessage('');
  renderDeviceTopicPreview();
  overlay.hidden = false;
  $('deviceId').focus();
}

function closeDevice() {
  const overlay = $('deviceOverlay');
  if (overlay) overlay.hidden = true;
}

function deviceMessage(text, ok) {
  const el = $('deviceMessage');
  if (!el) return;
  el.className = 'export-message' + (text ? (ok ? ' ok' : ' error') : '');
  el.textContent = text || '';
}

async function saveDevice() {
  const id = $('deviceId').value.trim();
  const label = $('deviceLabel').value.trim();
  const latRaw = $('deviceLat').value.trim();
  const lonRaw = $('deviceLon').value.trim();

  if (!id) {
    deviceMessage('Inserisci l\'ID del dispositivo.', false);
    return;
  }
  if ((latRaw && !lonRaw) || (!latRaw && lonRaw)) {
    deviceMessage('Inserisci entrambe le coordinate.', false);
    return;
  }

  const body = { id: id, label: label };
  if (latRaw) {
    const lat = Number(latRaw);
    const lon = Number(lonRaw);
    if (!Number.isFinite(lat) || !Number.isFinite(lon)) {
      deviceMessage('Coordinate non valide.', false);
      return;
    }
    body.lat = lat;
    body.lon = lon;
  }

  const button = $('deviceSave');
  button.disabled = true;
  try {
    const res = await fetch('/api/devices', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || res.statusText);

    closeDevice();
    configMessage('Sensore "' + (data.label || id) + '" aggiunto.', true);
    await Promise.all([loadDevices(), loadStats()]);
    render();
  } catch (err) {
    deviceMessage(err.message, false);
  } finally {
    button.disabled = false;
  }
}

function initConfig() {
  if ($('mqttApply')) $('mqttApply').addEventListener('click', applySettings);
  if ($('deviceAddBtn')) $('deviceAddBtn').addEventListener('click', openDevice);
  if ($('deviceSave')) $('deviceSave').addEventListener('click', saveDevice);
  if ($('deviceCancel')) $('deviceCancel').addEventListener('click', closeDevice);
  if ($('deviceClose')) $('deviceClose').addEventListener('click', closeDevice);
  if ($('deviceOverlay')) {
    $('deviceOverlay').addEventListener('click', e => {
      if (e.target === $('deviceOverlay')) closeDevice();
    });
  }

  const prefix = $('topicPrefixInput');
  if (prefix) {
    prefix.addEventListener('input', () => {
      updateTopicDisplay();
      renderDeviceTopicPreview();
    });
  }
  const sensor = $('deviceSelect');
  if (sensor) sensor.addEventListener('change', updateTopicDisplay);
  const id = $('deviceId');
  if (id) id.addEventListener('input', renderDeviceTopicPreview);

  document.addEventListener('keydown', e => {
    if (e.key === 'Escape' && !$('deviceOverlay').hidden) closeDevice();
  });

  [$('deviceId'), $('deviceLabel'), $('deviceLat'), $('deviceLon')].forEach(el => {
    if (el) {
      el.addEventListener('keydown', e => {
        if (e.key === 'Enter') saveDevice();
      });
    }
  });
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', initConfig);
} else {
  initConfig();
}

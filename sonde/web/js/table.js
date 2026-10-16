/* Valori correnti e tabella delle misure. */

function gaugeCard(label, value, unit) {
  return '<div class="gauge-card">' +
    '<div class="gauge-label">' + escapeHtml(label) + '</div>' +
    '<div class="gauge-value">' + value + '</div>' +
    '<div class="gauge-unit">' + escapeHtml(unit || '') + '</div>' +
    '</div>';
}

function kindBadge(kind) {
  const cls = kind === 'livello' ? 'level' : (kind === 'aria' || kind === 'misto' ? 'air' : '');
  return '<span class="badge ' + cls + '">' + escapeHtml(kind || '—') + '</span>';
}

function renderCurrentValues() {
  const container = $('currentValues');
  const nameEl = $('currentDeviceName');
  if (!container) return;

  if (selectedDevice === ALL) {
    const list = visibleDevices();
    if (nameEl) nameEl.textContent = 'Tutti i dispositivi (' + list.length + ')';
    if (list.length === 0) {
      container.innerHTML = '<div class="empty">Nessun dispositivo ricevuto.</div>';
      return;
    }
    container.innerHTML = list.map(d => {
      const m = latestByDevice[d.id];
      const metric = mainMetric(m);
      const when = m ? fmtTimeShort(m.ts) : 'nessun dato';
      return gaugeCard(d.label, metric.value, metric.unit + ' · ' + when);
    }).join('');
    return;
  }

  const d = deviceById(selectedDevice);
  const m = latestByDevice[selectedDevice] || readings.filter(r => r.device_id === selectedDevice).pop();
  if (nameEl) nameEl.textContent = d ? d.label : selectedDevice;

  if (!m) {
    container.innerHTML = '<div class="empty">Nessuna misura per questo dispositivo.</div>';
    return;
  }

  const cards = [];
  const kind = m.kind || (d && d.kind) || '';
  const showLevel = kind === 'livello' || kind === 'misto';
  const showAir = kind === 'aria' || kind === 'misto';

  if (showLevel) {
    cards.push(gaugeCard('Livello', fmt(m.water_level_cm, 1), 'cm'));
    cards.push(gaugeCard('Ultrasuoni', fmt(m.ultrasonic_cm, 1), 'cm'));
    cards.push(gaugeCard('ToF', fmt(m.tof_cm, 1), 'cm'));
  }
  if (showAir) {
    cards.push(gaugeCard('PM1', fmt(m.pm1, 0), 'µg/m³'));
    cards.push(gaugeCard('PM2.5', fmt(m.pm25, 0), 'µg/m³'));
    cards.push(gaugeCard('PM10', fmt(m.pm10, 0), 'µg/m³'));
    cards.push(gaugeCard('Temperatura', fmt(temperature(m), 1), '°C'));
    cards.push(gaugeCard('Umidità', fmt(humidity(m), 0), '%'));
    cards.push(gaugeCard('Pressione', fmt(pressureHpa(m), 1), 'hPa'));
  }
  cards.push(gaugeCard('Latitudine', fmt(m.lat, 5), ''));
  cards.push(gaugeCard('Longitudine', fmt(m.lon, 5), ''));
  if (m.rssi !== null && m.rssi !== undefined) cards.push(gaugeCard('RSSI', fmtInt(m.rssi), m.net || ''));

  container.innerHTML = cards.join('');
}

function renderTable() {
  const tbody = $('tableBody');
  if (!tbody) return;

  const rows = visibleReadings().slice(-100).reverse();
  if (rows.length === 0) {
    tbody.innerHTML = '<tr><td colspan="13" style="text-align:center;padding:20px;color:var(--muted);">Nessuna misura ricevuta</td></tr>';
    return;
  }

  tbody.innerHTML = rows.map(m => {
    return '<tr' + (m.exported ? ' class="exported"' : '') + '>' +
      '<td>' + escapeHtml(fmtTime(m.ts)) + '</td>' +
      '<td>' + escapeHtml(m.device_label || m.device_id) + '</td>' +
      '<td>' + kindBadge(m.kind) + '</td>' +
      '<td>' + fmt(m.water_level_cm, 1) + '</td>' +
      '<td>' + fmt(m.pm1, 0) + '</td>' +
      '<td>' + fmt(m.pm25, 0) + '</td>' +
      '<td>' + fmt(m.pm10, 0) + '</td>' +
      '<td>' + fmt(temperature(m), 1) + '</td>' +
      '<td>' + fmt(humidity(m), 0) + '</td>' +
      '<td>' + fmt(pressureHpa(m), 1) + '</td>' +
      '<td>' + fmt(m.lat, 4) + '</td>' +
      '<td>' + fmt(m.lon, 4) + '</td>' +
      '<td>' + (m.exported ? '<span class="badge">Sì</span>' : '') + '</td>' +
      '</tr>';
  }).join('');
}

/* Pannello di esportazione con query SQL. */

function downloadBlob(blob, filename) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

function openExportMenu() {
  $('exportOverlay').hidden = false;
  if (!$('exportSQL').value.trim()) $('exportSQL').value = DEFAULT_SQL;
  $('exportMessage').textContent = '';
  $('exportMessage').className = 'export-message';
  $('previewResult').innerHTML = '';
  loadSchema();
}

function closeExportMenu() {
  $('exportOverlay').hidden = true;
}

async function loadSchema() {
  const list = $('schemaList');
  if (!list || list.dataset.loaded) return;
  try {
    const out = await fetchJSON('/api/schema');
    list.dataset.loaded = '1';
    list.innerHTML = (out.columns || []).map(c =>
      '<code title="' + escapeHtml(c.type) + '">' + escapeHtml(c.name) + '</code>'
    ).join('');
    list.querySelectorAll('code').forEach(el => {
      el.addEventListener('click', () => insertSQL(el.textContent));
    });
  } catch (e) {
    list.textContent = 'Schema non disponibile.';
  }
}

function insertSQL(token) {
  const ta = $('exportSQL');
  const start = ta.selectionStart || ta.value.length;
  const end = ta.selectionEnd || ta.value.length;
  ta.value = ta.value.slice(0, start) + token + ta.value.slice(end);
  ta.focus();
  ta.selectionStart = ta.selectionEnd = start + token.length;
}

/* CSV+JSON insieme oppure ZIP richiedono un unico download. */
function chosenFormat() {
  const csv = $('fmtCSV').checked;
  const json = $('fmtJSON').checked;
  const zip = $('fmtZIP').checked;
  if (zip || (csv && json)) return 'zip';
  if (csv) return 'csv';
  if (json) return 'json';
  return null;
}

function renderPreview(out) {
  const cols = out.columns || [];
  const rows = out.rows || [];
  if (rows.length === 0) {
    $('previewResult').innerHTML = '<div class="empty">Nessuna riga.</div>';
    return;
  }
  const head = '<tr>' + cols.map(c => '<th>' + escapeHtml(c) + '</th>').join('') + '</tr>';
  const body = rows.map(r =>
    '<tr>' + cols.map(c => '<td>' + escapeHtml(r[c]) + '</td>').join('') + '</tr>'
  ).join('');
  $('previewResult').innerHTML = '<table><thead>' + head + '</thead><tbody>' + body + '</tbody></table>';
}

async function previewExport() {
  const msg = $('exportMessage');
  msg.className = 'export-message';
  msg.textContent = 'Esecuzione...';
  try {
    const sql = $('exportSQL').value.trim();
    const out = await fetchJSON('/api/query?limit=100&sql=' + encodeURIComponent(sql));
    msg.className = 'export-message ok';
    msg.textContent = out.rows.length + ' righe mostrate su ' + out.count + ' totali.';
    renderPreview(out);
  } catch (e) {
    msg.className = 'export-message error';
    msg.textContent = e.message;
    $('previewResult').innerHTML = '';
  }
}

async function downloadExport() {
  const msg = $('exportMessage');
  const format = chosenFormat();
  if (!format) {
    msg.className = 'export-message error';
    msg.textContent = 'Seleziona almeno un formato (CSV, JSON o ZIP).';
    return;
  }

  const sql = $('exportSQL').value.trim();
  const mark = $('markExported').checked ? '1' : '0';

  msg.className = 'export-message';
  msg.textContent = 'Preparazione del file...';
  try {
    const url = '/api/export?format=' + format + '&mark=' + mark + '&sql=' + encodeURIComponent(sql);
    const r = await fetch(url);
    if (!r.ok) {
      let m = r.statusText;
      try { m = (await r.json()).error || m; } catch (e) {}
      throw new Error(m);
    }
    const blob = await r.blob();
    const cd = r.headers.get('Content-Disposition') || '';
    const match = /filename="([^"]+)"/.exec(cd);
    downloadBlob(blob, match ? match[1] : ('sonde_export.' + format));

    msg.className = 'export-message ok';
    msg.textContent = 'Esportazione completata' + (mark === '1' ? ', dati segnati come esportati.' : '.');

    await Promise.all([loadStats(), loadReadings(), loadLatest()]);
    render();
  } catch (e) {
    msg.className = 'export-message error';
    msg.textContent = e.message;
  }
}

async function clearExported() {
  if (!confirm('Cancellare tutte le misure gi\u00e0 esportate?')) return;
  try {
    const r = await fetch('/api/clear-exported', { method: 'POST' });
    if (!r.ok) throw new Error((await r.json()).error || r.statusText);
    const out = await r.json();
    await Promise.all([loadStats(), loadReadings(), loadLatest()]);
    render();
    alert('Eliminate ' + (out.deleted || 0) + ' misure esportate.');
  } catch (e) {
    alert('Errore: ' + e.message);
  }
}

function initExportUI() {
  $('exportBtn').addEventListener('click', openExportMenu);
  $('exportClose').addEventListener('click', closeExportMenu);
  $('exportOverlay').addEventListener('click', e => {
    if (e.target === $('exportOverlay')) closeExportMenu();
  });
  $('previewBtn').addEventListener('click', previewExport);
  $('downloadBtn').addEventListener('click', downloadExport);
  $('clearExported').addEventListener('click', clearExported);

  document.addEventListener('keydown', e => {
    if (e.key === 'Escape' && !$('exportOverlay').hidden) closeExportMenu();
  });
}

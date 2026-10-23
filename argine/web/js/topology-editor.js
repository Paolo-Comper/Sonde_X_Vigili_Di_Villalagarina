/* Editor visuale dei collegamenti di affluenza (topology.json). */
(function () {
  function options(selected) {
    const labels = devices.map(d => d.label);
    let html = '';
    if (selected && !labels.includes(selected)) {
      html += '<option value="' + escapeHtml(selected) + '" selected>' +
        escapeHtml(selected) + ' (non disponibile)</option>';
    }
    devices.forEach(d => {
      html += '<option value="' + escapeHtml(d.label) + '"' +
        (d.label === selected ? ' selected' : '') + '>' +
        escapeHtml(d.label) + '</option>';
    });
    return html;
  }

  function showEmpty() {
    const rows = $('topologyRows');
    if (!rows) return;
    rows.innerHTML = '<div class="topology-empty">Nessun collegamento. Aggiungine uno.</div>';
  }

  function addRow(link) {
    const rows = $('topologyRows');
    if (!rows || devices.length === 0) return;
    const empty = rows.querySelector('.topology-empty');
    if (empty) empty.remove();

    const defFrom = devices[0] ? devices[0].label : '';
    const defTo = devices[1] ? devices[1].label : defFrom;
    const from = link ? link.from : defFrom;
    const to = link ? link.to : defTo;

    const row = document.createElement('div');
    row.className = 'topology-row';
    row.innerHTML =
      '<select class="topo-from" aria-label="Sensore a monte">' + options(from) + '</select>' +
      '<span class="topology-arrow">&rarr;</span>' +
      '<select class="topo-to" aria-label="Sensore a valle">' + options(to) + '</select>' +
      '<button class="btn btn-ghost topo-remove" type="button" title="Rimuovi">&#10005;</button>';

    row.querySelector('.topo-remove').addEventListener('click', () => {
      row.remove();
      if (!rows.querySelector('.topology-row')) showEmpty();
    });
    rows.appendChild(row);
  }

  function open() {
    const overlay = $('topologyOverlay');
    const rows = $('topologyRows');
    if (!overlay || !rows) return;

    rows.innerHTML = '';
    const message = $('topologyMessage');
    if (message) {
      message.textContent = '';
      message.className = 'export-message';
    }

    if (devices.length === 0) {
      rows.innerHTML = '<div class="topology-empty">Nessun dispositivo ancora ricevuto.</div>';
    } else {
      const links = topology.links || [];
      if (links.length === 0) showEmpty();
      links.forEach(addRow);
    }
    overlay.hidden = false;
  }

  function close() {
    const overlay = $('topologyOverlay');
    if (overlay) overlay.hidden = true;
  }

  function collect() {
    const out = [];
    document.querySelectorAll('#topologyRows .topology-row').forEach(row => {
      const from = row.querySelector('.topo-from').value;
      const to = row.querySelector('.topo-to').value;
      if (from && to) out.push({ from, to });
    });
    return out;
  }

  async function save() {
    const message = $('topologyMessage');
    const button = $('topologySave');
    const links = collect();

    for (const link of links) {
      if (link.from === link.to) {
        message.className = 'export-message error';
        message.textContent = 'Un sensore non puo\' collegarsi a se stesso.';
        return;
      }
    }

    button.disabled = true;
    try {
      const res = await fetch('/api/topology', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ links })
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || res.statusText);

      topology = { links: data.links || [] };
      message.className = 'export-message ok';
      message.textContent = 'Salvato.';
      render();
      setTimeout(close, 350);
    } catch (err) {
      message.className = 'export-message error';
      message.textContent = err.message;
    } finally {
      button.disabled = false;
    }
  }

  function init() {
    document.querySelectorAll('.topology-edit').forEach(btn =>
      btn.addEventListener('click', open));
    if ($('topologyAdd')) $('topologyAdd').addEventListener('click', () => addRow(null));
    if ($('topologySave')) $('topologySave').addEventListener('click', save);
    if ($('topologyCancel')) $('topologyCancel').addEventListener('click', close);
    if ($('topologyClose')) $('topologyClose').addEventListener('click', close);
    if ($('topologyOverlay')) {
      $('topologyOverlay').addEventListener('click', e => {
        if (e.target === $('topologyOverlay')) close();
      });
    }
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();

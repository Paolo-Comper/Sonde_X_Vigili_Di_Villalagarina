/* Grafo delle affluenze: alternativa alla mappa quando questa non e'
   disponibile (es. nessuna connessione per le tile) e vista d'insieme. */

// Costruisce nodi e archi risolvendo from/to per label oppure per id.
function graphModel() {
  const nodes = [];
  const alias = new Map();

  function add(label, deviceId) {
    const node = { key: String(label).toLowerCase(), label: label, deviceId: deviceId || null, edgesTo: [] };
    nodes.push(node);
    alias.set(node.key, node);
    if (deviceId) alias.set(String(deviceId).toLowerCase(), node);
    return node;
  }

  function resolve(ref) {
    const k = String(ref || '').toLowerCase();
    if (alias.has(k)) return alias.get(k);
    const node = { key: k, label: String(ref), deviceId: null, edgesTo: [] };
    nodes.push(node);
    alias.set(k, node);
    return node;
  }

  devices.forEach(d => add(d.label, d.id));
  (topology.nodes || []).forEach(n => add(n.label || n.id, null));

  const edges = [];
  (topology.links || []).forEach(l => {
    const from = resolve(l.from);
    const to = resolve(l.to);
    from.edgesTo.push(to.key);
    edges.push({ from: from.key, to: to.key });
  });

  return { nodes, edges };
}

// Assegna un livello a ogni nodo (longest path) per disporli da monte a valle.
function graphLayers(nodes, edges) {
  const layer = new Map();
  const incoming = new Map();
  nodes.forEach(n => incoming.set(n.key, 0));
  edges.forEach(e => incoming.set(e.to, (incoming.get(e.to) || 0) + 1));

  const byKey = new Map(nodes.map(n => [n.key, n]));
  const remaining = new Map(incoming);
  const queue = nodes.filter(n => !incoming.get(n.key)).map(n => n.key);
  queue.forEach(k => layer.set(k, 0));

  for (let i = 0; i < queue.length; i++) {
    const node = byKey.get(queue[i]);
    const base = layer.get(node.key) || 0;
    node.edgesTo.forEach(t => {
      layer.set(t, Math.max(layer.get(t) || 0, base + 1));
      remaining.set(t, (remaining.get(t) || 0) - 1);
      if (remaining.get(t) === 0 && queue.indexOf(t) === -1) queue.push(t);
    });
  }
  nodes.forEach(n => { if (!layer.has(n.key)) layer.set(n.key, 0); });
  return layer;
}

function graphNodeValue(node) {
  if (!node.deviceId) return '';
  const m = latestByDevice[node.deviceId];
  if (!m) return '';
  const metric = mainMetric(m);
  if (metric.value === undefined || metric.value === null) return '';
  return metric.value + (metric.unit ? ' ' + metric.unit : '');
}

function renderGraphSVG(container) {
  const model = graphModel();
  if (model.nodes.length === 0) {
    container.innerHTML = '<div class="empty">Nessun collegamento configurato.</div>';
    return;
  }

  const layerOf = graphLayers(model.nodes, model.edges);
  const byLayer = [];
  model.nodes.forEach(n => {
    const l = layerOf.get(n.key) || 0;
    (byLayer[l] = byLayer[l] || []).push(n);
  });
  byLayer.forEach(col => col.sort((a, b) => String(a.label).localeCompare(String(b.label), 'it')));

  const W = Math.max(container.clientWidth || 900, 480);
  const H = 360;
  const padX = 80;
  const padY = 46;
  const nLayers = byLayer.length;
  const xOf = i => nLayers <= 1 ? W / 2 : padX + i * (W - 2 * padX) / (nLayers - 1);

  const pos = new Map();
  byLayer.forEach((col, i) => {
    const x = xOf(i);
    col.forEach((node, j) => {
      pos.set(node.key, { x: x, y: padY + (j + 1) * (H - 2 * padY) / (col.length + 1) });
    });
  });

  const r = 24;
  let edgesSvg = '';
  model.edges.forEach(e => {
    const a = pos.get(e.from);
    const b = pos.get(e.to);
    if (!a || !b) return;
    const dx = b.x - a.x;
    const dy = b.y - a.y;
    const len = Math.hypot(dx, dy) || 1;
    const ux = dx / len;
    const uy = dy / len;
    const x1 = a.x + ux * r;
    const y1 = a.y + uy * r;
    const x2 = b.x - ux * (r + 8);
    const y2 = b.y - uy * (r + 8);
    edgesSvg += '<line x1="' + x1 + '" y1="' + y1 + '" x2="' + x2 + '" y2="' + y2 +
      '" stroke="#e74c3c" stroke-width="1.6" stroke-opacity="0.6" marker-end="url(#sonde-graph-arrow)" />';
  });

  let nodesSvg = '';
  model.nodes.forEach(n => {
    const p = pos.get(n.key);
    if (!p) return;
    const fill = n.deviceId ? '#2f7fd6' : '#8e44ad';
    const value = graphNodeValue(n);
    nodesSvg += '<circle cx="' + p.x + '" cy="' + p.y + '" r="' + r + '" fill="' + fill +
      '" stroke="#ffffff" stroke-width="2" />';
    if (value) {
      nodesSvg += '<text x="' + p.x + '" y="' + (p.y + 4) + '" text-anchor="middle" fill="#ffffff" ' +
        'font-size="10" font-weight="600">' + escapeHtml(value) + '</text>';
    }
    nodesSvg += '<text x="' + p.x + '" y="' + (p.y + r + 15) + '" text-anchor="middle" ' +
      'fill="currentColor" font-size="11">' + escapeHtml(n.label) + '</text>';
  });

  container.innerHTML =
    '<svg class="graph-svg" viewBox="0 0 ' + W + ' ' + H + '" width="100%" height="' + H + '" role="img">' +
    '<defs><marker id="sonde-graph-arrow" viewBox="0 0 10 10" refX="9" refY="5" ' +
    'markerWidth="7" markerHeight="7" orient="auto"><path d="M0,0 L10,5 L0,10 z" fill="#e74c3c" /></marker></defs>' +
    edgesSvg + nodesSvg + '</svg>';
}

function updateGraph() {
  const section = $('graphSection');
  const container = $('graphContainer');
  if (!section || !container) return;

  const show = selectedDevice === ALL && (topology.links || []).length > 0;
  section.hidden = !show;
  if (!show) return;
  renderGraphSVG(container);
}

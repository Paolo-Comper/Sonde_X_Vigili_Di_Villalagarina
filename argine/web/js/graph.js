/* Grafo delle affluenze: alternativa alla mappa quando questa non e'
   disponibile (es. nessuna connessione per le tile) e vista d'insieme. */

// Costruisce nodi e archi risolvendo from/to per label oppure per id.
// I nodi sono solo sensori reali: i collegamenti verso punti senza sensore (es. il
// fiume) vengono ignorati, mentre il sensore resta comunque nel grafo.
function graphModel() {
  const nodes = devices.map(d => ({
    key: String(d.label).toLowerCase(),
    label: d.label,
    deviceId: d.id,
    edgesTo: []
  }));

  const alias = new Map();
  nodes.forEach(n => {
    alias.set(n.key, n);
    alias.set(String(n.deviceId).toLowerCase(), n);
  });

  const edges = [];
  (topology.links || []).forEach(l => {
    const from = alias.get(String(l.from || '').toLowerCase());
    const to = alias.get(String(l.to || '').toLowerCase());
    if (!from || !to) return;
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

function graphNodeWidth(node) {
  const value = graphNodeValue(node);
  const longest = Math.max(String(node.label || '').length, value.length + 2);
  return Math.max(170, Math.min(320, 58 + longest * 7.4));
}

function graphLabel(node) {
  const label = String(node.label || '');
  return label.length > 30 ? label.slice(0, 29) + '…' : label;
}

// Flowchart: nodi arrotondati su colonne (monte -> valle) e archi curvi.
function renderGraphSVG(container) {
  const model = graphModel();
  if (model.nodes.length === 0) {
    container.innerHTML = '<div class="empty">Nessun dispositivo disponibile.</div>';
    return;
  }

  const layerOf = graphLayers(model.nodes, model.edges);
  const byLayer = [];
  model.nodes.forEach(n => {
    const l = layerOf.get(n.key) || 0;
    (byLayer[l] = byLayer[l] || []).push(n);
  });
  const columns = byLayer.filter(col => col && col.length > 0);
  columns.forEach(col => col.sort((a, b) => String(a.label).localeCompare(String(b.label), 'it')));

  const nodeH = 62;
  const gapX = 96;
  const gapY = 28;
  const padX = 30;
  const padY = 34;

  const widths = columns.map(col => Math.max(170, ...col.map(graphNodeWidth)));
  const colX = [];
  let cursor = padX;
  widths.forEach(w => { colX.push(cursor); cursor += w + gapX; });
  const contentW = Math.max(cursor - gapX + padX, 420);

  const tallest = Math.max(1, ...columns.map(col => col.length * nodeH + (col.length - 1) * gapY));
  const contentH = tallest + padY * 2;

  const pos = new Map();
  columns.forEach((col, li) => {
    const colH = col.length * nodeH + (col.length - 1) * gapY;
    const y0 = padY + (tallest - colH) / 2;
    col.forEach((node, i) => {
      pos.set(node.key, {
        x: colX[li],
        y: y0 + i * (nodeH + gapY),
        w: graphNodeWidth(node),
        h: nodeH
      });
    });
  });

  let edgesSvg = '';
  model.edges.forEach(e => {
    const a = pos.get(e.from);
    const b = pos.get(e.to);
    if (!a || !b) return;
    const x1 = a.x + a.w;
    const y1 = a.y + a.h / 2;
    const x2 = b.x - 3;
    const y2 = b.y + b.h / 2;
    let d;
    if (x2 > x1 + 12) {
      const dx = Math.max(44, (x2 - x1) * 0.5);
      d = `M ${x1} ${y1} C ${x1 + dx} ${y1}, ${x2 - dx} ${y2}, ${x2} ${y2}`;
    } else {
      // Arco di ritorno (ciclo o stesso livello): passa sopra i nodi.
      const lift = Math.max(46, Math.abs(y2 - y1) * 0.3 + 40);
      d = `M ${x1} ${y1} C ${x1 + 64} ${y1 - lift}, ${x2 - 64} ${y2 - lift}, ${x2} ${y2}`;
    }
    edgesSvg += `<path class="flow-edge" d="${d}" marker-end="url(#argine-graph-arrow)" />`;
  });

  let nodesSvg = '';
  model.nodes.forEach(n => {
    const p = pos.get(n.key);
    if (!p) return;
    const value = graphNodeValue(n);
    const centerY = p.y + p.h / 2;
    nodesSvg +=
      `<g class="flow-node">` +
        `<rect x="${p.x}" y="${p.y}" width="${p.w}" height="${p.h}" rx="13" />` +
        `<circle class="flow-dot" cx="${p.x + 21}" cy="${centerY}" r="5.5" />` +
        `<text class="flow-title" x="${p.x + 36}" y="${value ? p.y + 26 : centerY + 5}">` +
          `${escapeHtml(graphLabel(n))}</text>` +
        (value
          ? `<text class="flow-value" x="${p.x + 36}" y="${p.y + 45}">${escapeHtml(value)}</text>`
          : '') +
      `</g>`;
  });

  container.innerHTML =
    `<div class="graph-scroll">` +
      `<svg class="graph-svg" viewBox="0 0 ${contentW} ${contentH}" width="100%" ` +
        `role="img" aria-label="Grafo delle affluenze">` +
        `<defs><marker id="argine-graph-arrow" viewBox="0 0 10 10" refX="9" refY="5" ` +
          `markerWidth="7" markerHeight="7" orient="auto">` +
          `<path d="M 0 0 L 10 5 L 0 10 z" /></marker></defs>` +
        edgesSvg + nodesSvg +
      `</svg>` +
    `</div>`;
}

function updateGraph() {
  const section = $('graphSection');
  const container = $('graphContainer');
  if (!section || !container) return;

  const show = selectedDevice === ALL && devices.length > 0;
  section.hidden = !show;
  if (!show) return;
  renderGraphSVG(container);
}

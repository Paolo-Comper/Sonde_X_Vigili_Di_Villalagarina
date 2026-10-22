/* Mappa MapLibre con tutti i dispositivi. */

let sondeMap = null;
let mapFitted = false;
let lastSelection = null;
let mapDirty = false;
let deviceEventsBound = false;

const MAP_STYLES = {
  light: 'https://basemaps.cartocdn.com/gl/positron-gl-style/style.json',
  dark: 'https://basemaps.cartocdn.com/gl/dark-matter-gl-style/style.json'
};

function mapStyle() {
  return document.documentElement.getAttribute('data-theme') === 'dark'
    ? MAP_STYLES.dark : MAP_STYLES.light;
}

function emptyFC() {
  return { type: 'FeatureCollection', features: [] };
}

function deviceFeatures() {
  const features = [];
  devices.forEach(d => {
    const m = latestByDevice[d.id];
    const lat = (m && m.lat !== null && m.lat !== undefined) ? m.lat : d.lat;
    const lon = (m && m.lon !== null && m.lon !== undefined) ? m.lon : d.lon;
    if (lat === null || lat === undefined || lon === null || lon === undefined) return;
    features.push({
      type: 'Feature',
      properties: {
        id: d.id,
        label: d.label,
        selected: d.id === selectedDevice,
        level: m ? m.water_level_cm : null,
        pm25: m ? m.pm25 : null,
        temp: m ? temperature(m) : null,
        hum: m ? humidity(m) : null,
        press: m ? pressureHpa(m) : null,
        ts: m ? m.ts : null
      },
      geometry: { type: 'Point', coordinates: [lon, lat] }
    });
  });
  // Il dispositivo selezionato viene disegnato per ultimo, sopra gli altri.
  features.sort((a, b) => (a.properties.selected ? 1 : 0) - (b.properties.selected ? 1 : 0));
  return { type: 'FeatureCollection', features };
}

/* Risolve un riferimento (id o label) solo tra le sonde reali: un collegamento
   verso un punto senza sonda viene ignorato, per non disegnare frecce nel vuoto. */
function nodeCoord(key) {
  const k = String(key || '').toLowerCase();
  for (const d of devices) {
    if (String(d.id).toLowerCase() !== k && String(d.label).toLowerCase() !== k) continue;
    const m = latestByDevice[d.id];
    const lat = (m && m.lat !== null && m.lat !== undefined) ? m.lat : d.lat;
    const lon = (m && m.lon !== null && m.lon !== undefined) ? m.lon : d.lon;
    if (lat === null || lat === undefined || lon === null || lon === undefined) return null;
    return { lat, lon, label: d.label };
  }
  return null;
}

function bearingDeg(a, b) {
  const toRad = Math.PI / 180;
  const lat1 = a.lat * toRad;
  const lat2 = b.lat * toRad;
  const dLon = (b.lon - a.lon) * toRad;
  const y = Math.sin(dLon) * Math.cos(lat2);
  const x = Math.cos(lat1) * Math.sin(lat2) - Math.sin(lat1) * Math.cos(lat2) * Math.cos(dLon);
  return (Math.atan2(y, x) * 180 / Math.PI + 360) % 360;
}

// Tratti e punte di freccia dei collegamenti di affluenza.
function linkFeatures() {
  const features = [];
  (topology.links || []).forEach(l => {
    const a = nodeCoord(l.from);
    const b = nodeCoord(l.to);
    if (!a || !b) return;
    features.push({
      type: 'Feature',
      properties: { kind: 'line' },
      geometry: { type: 'LineString', coordinates: [[a.lon, a.lat], [b.lon, b.lat]] }
    });
    // Punta poco prima della destinazione, per non finire sotto il marker.
    const t = 0.82;
    features.push({
      type: 'Feature',
      properties: { kind: 'arrow', bearing: bearingDeg(a, b) },
      geometry: {
        type: 'Point',
        coordinates: [a.lon + (b.lon - a.lon) * t, a.lat + (b.lat - a.lat) * t]
      }
    });
  });
  return { type: 'FeatureCollection', features };
}

// Punta di freccia rivolta a nord; la rotazione la applica icon-rotate.
function arrowIcon() {
  const size = 28;
  const canvas = document.createElement('canvas');
  canvas.width = size;
  canvas.height = size;
  const ctx = canvas.getContext('2d');
  ctx.fillStyle = '#e74c3c';
  ctx.beginPath();
  ctx.moveTo(size / 2, 2);
  ctx.lineTo(size - 5, size - 3);
  ctx.lineTo(size / 2, size - 9);
  ctx.lineTo(5, size - 3);
  ctx.closePath();
  ctx.fill();
  return { width: size, height: size, data: ctx.getImageData(0, 0, size, size).data };
}

function addLinkLayers() {
  if (!sondeMap || sondeMap.getSource('links')) return;
  sondeMap.addSource('links', { type: 'geojson', data: emptyFC() });
  sondeMap.addLayer({
    id: 'links-line',
    type: 'line',
    source: 'links',
    filter: ['==', ['get', 'kind'], 'line'],
    paint: {
      'line-color': '#e74c3c',
      'line-width': 1.6,
      'line-opacity': 0.55,
      'line-dasharray': [2, 2]
    }
  });
  if (!sondeMap.hasImage('sonde-arrow')) sondeMap.addImage('sonde-arrow', arrowIcon());
  sondeMap.addLayer({
    id: 'links-arrow',
    type: 'symbol',
    source: 'links',
    filter: ['==', ['get', 'kind'], 'arrow'],
    layout: {
      'icon-image': 'sonde-arrow',
      'icon-size': ['interpolate', ['linear'], ['zoom'], 9, 0.5, 14, 0.85],
      'icon-rotate': ['get', 'bearing'],
      'icon-rotation-alignment': 'map',
      'icon-allow-overlap': true,
      'icon-ignore-placement': true
    }
  });
}

function updateLinks() {
  if (!sondeMap) return;
  const src = sondeMap.getSource('links');
  if (!src) return;
  src.setData(linkFeatures());
  const visibility = selectedDevice === ALL ? 'visible' : 'none';
  ['links-line', 'links-arrow'].forEach(id => {
    if (sondeMap.getLayer(id)) sondeMap.setLayoutProperty(id, 'visibility', visibility);
  });
}

function bindDeviceEvents() {
  if (deviceEventsBound || !sondeMap) return;
  deviceEventsBound = true;

  sondeMap.on('click', 'devices', e => {
    if (!e.features || e.features.length === 0) return;
    const p = e.features[0].properties;
    const lines = ['<strong>' + escapeHtml(p.label) + '</strong>'];
    if (p.level !== null && p.level !== undefined) lines.push('Livello: ' + fmt(p.level, 1) + ' cm');
    if (p.pm25 !== null && p.pm25 !== undefined) lines.push('PM2.5: ' + fmt(p.pm25, 0) + ' µg/m³');
    if (p.temp !== null && p.temp !== undefined) lines.push('Temp: ' + fmt(p.temp, 1) + ' °C');
    if (p.hum !== null && p.hum !== undefined) lines.push('Umidità: ' + fmt(p.hum, 0) + ' %');
    if (p.press !== null && p.press !== undefined) lines.push('Pressione: ' + fmt(p.press, 1) + ' hPa');
    if (p.ts) lines.push('Ora: ' + escapeHtml(fmtTime(p.ts)));
    new maplibregl.Popup({ className: 'sonde-popup', maxWidth: '300px' })
      .setLngLat(e.lngLat)
      .setHTML(lines.join('<br>'))
      .addTo(sondeMap);
  });
  sondeMap.on('mouseenter', 'devices', () => { sondeMap.getCanvas().style.cursor = 'pointer'; });
  sondeMap.on('mouseleave', 'devices', () => { sondeMap.getCanvas().style.cursor = ''; });
}

// Assicura che sorgenti e layer esistano anche dopo un setStyle (tema).
function ensureMapLayers() {
  if (!sondeMap || !sondeMap.isStyleLoaded()) return false;
  // I collegamenti stanno sotto ai marker dei dispositivi.
  addLinkLayers();
  if (!sondeMap.getSource('devices')) {
    sondeMap.addSource('devices', { type: 'geojson', data: emptyFC() });
    sondeMap.addLayer({
      id: 'devices',
      type: 'circle',
      source: 'devices',
      paint: {
        'circle-radius': ['case', ['get', 'selected'], 13, 8],
        'circle-color': ['case', ['get', 'selected'], '#e74c3c', '#2f7fd6'],
        'circle-stroke-width': ['case', ['get', 'selected'], 3, 2],
        'circle-stroke-color': '#ffffff',
        'circle-opacity': ['case', ['get', 'selected'], 1, 0.85]
      }
    });
  }
  bindDeviceEvents();
  return true;
}

function addMapLayers() {
  if (!ensureMapLayers()) { mapDirty = true; return; }
  updateMap();
}

function updateMap() {
  if (!sondeMap) return;
  if (!ensureMapLayers()) { mapDirty = true; return; }
  mapDirty = false;

  const src = sondeMap.getSource('devices');
  if (!src) { mapDirty = true; return; }

  const fc = deviceFeatures();
  src.setData(fc);

  updateLinks();

  if (selectedDevice !== ALL) {
    // Un dispositivo specifico: centra la mappa su di lui (solo al cambio).
    const target = fc.features.find(f => f.properties.selected);
    if (target && lastSelection !== selectedDevice) {
      sondeMap.easeTo({
        center: target.geometry.coordinates,
        zoom: Math.max(sondeMap.getZoom(), 12),
        duration: 500
      });
    }
  } else if (!mapFitted || lastSelection !== ALL) {
    // "Tutti": inquadra l'insieme dei dispositivi.
    if (fc.features.length > 0) {
      mapFitted = true;
      const bounds = new maplibregl.LngLatBounds();
      fc.features.forEach(f => bounds.extend(f.geometry.coordinates));
      sondeMap.fitBounds(bounds, { padding: 60, maxZoom: 14, duration: 500 });
    }
  }
  lastSelection = selectedDevice;
}

function applyMapTheme() {
  if (!sondeMap) return;
  // setStyle rimuove sorgenti e layer: style.load li ricrea e riapplica i dati.
  mapDirty = true;
  sondeMap.setStyle(mapStyle());
}

function initMap() {
  const container = $('map');
  if (!container) return;
  if (typeof maplibregl === 'undefined') {
    container.innerHTML = '<div class="map-unavailable">Mappa non disponibile: serve una connessione a internet.</div>';
    return;
  }

  sondeMap = new maplibregl.Map({
    container: 'map',
    style: mapStyle(),
    center: [11.03, 45.91],
    zoom: 10
  });
  sondeMap.addControl(new maplibregl.NavigationControl(), 'top-right');
  sondeMap.on('load', addMapLayers);
  sondeMap.on('style.load', addMapLayers);
  // Se un aggiornamento arriva mentre lo stile non e' pronto, lo si applica qui.
  sondeMap.on('idle', () => { if (mapDirty) updateMap(); });

  if (typeof ResizeObserver !== 'undefined') {
    const observer = new ResizeObserver(() => { if (sondeMap) sondeMap.resize(); });
    observer.observe(container);
  }
  window.addEventListener('resize', () => { if (sondeMap) sondeMap.resize(); });
  document.addEventListener('visibilitychange', () => {
    if (document.hidden || !sondeMap) return;
    // Il canvas puo' aver perso le dimensioni mentre la scheda era nascosta.
    setTimeout(() => {
      if (!sondeMap) return;
      sondeMap.resize();
      updateMap();
    }, 80);
  });
}

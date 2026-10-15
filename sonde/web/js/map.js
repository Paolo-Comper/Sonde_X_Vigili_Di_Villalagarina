/* Mappa MapLibre con tutti i dispositivi. */

let sondeMap = null;
let mapFitted = false;

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
        kind: d.kind,
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
  return { type: 'FeatureCollection', features };
}

function addMapLayers() {
  if (!sondeMap || sondeMap.getSource('devices')) return;

  sondeMap.addSource('devices', { type: 'geojson', data: emptyFC() });
  sondeMap.addLayer({
    id: 'devices',
    type: 'circle',
    source: 'devices',
    paint: {
      'circle-radius': 9,
      'circle-color': [
        'match', ['get', 'kind'],
        'livello', '#2f7fd6',
        'aria', '#2ecc71',
        'misto', '#8e44ad',
        '#888888'
      ],
      'circle-stroke-width': 2,
      'circle-stroke-color': '#ffffff',
      'circle-opacity': 0.95
    }
  });
  sondeMap.on('click', 'devices', e => {
    const p = e.features[0].properties;
    const lines = [
      '<strong>' + escapeHtml(p.label) + '</strong>',
      'Tipo: ' + escapeHtml(p.kind)
    ];
    if (p.level !== null && p.level !== undefined) lines.push('Livello: ' + fmt(p.level, 1) + ' cm');
    if (p.pm25 !== null && p.pm25 !== undefined) lines.push('PM2.5: ' + fmt(p.pm25, 0) + ' µg/m³');
    if (p.temp !== null && p.temp !== undefined) lines.push('Temp: ' + fmt(p.temp, 1) + ' °C');
    if (p.hum !== null && p.hum !== undefined) lines.push('Umidità: ' + fmt(p.hum, 0) + ' %');
    if (p.press !== null && p.press !== undefined) lines.push('Pressione: ' + fmt(p.press, 1) + ' hPa');
    if (p.ts) lines.push('Ora: ' + escapeHtml(fmtTime(p.ts)));
    new maplibregl.Popup().setLngLat(e.lngLat).setHTML(lines.join('<br>')).addTo(sondeMap);
  });
  sondeMap.on('mouseenter', 'devices', () => { sondeMap.getCanvas().style.cursor = 'pointer'; });
  sondeMap.on('mouseleave', 'devices', () => { sondeMap.getCanvas().style.cursor = ''; });
}

function updateMap() {
  if (!sondeMap) return;
  const src = sondeMap.getSource('devices');
  if (!src) return;

  const fc = deviceFeatures();
  src.setData(fc);

  if (!mapFitted && fc.features.length > 0) {
    mapFitted = true;
    const bounds = new maplibregl.LngLatBounds();
    fc.features.forEach(f => bounds.extend(f.geometry.coordinates));
    sondeMap.fitBounds(bounds, { padding: 60, maxZoom: 14, duration: 0 });
  }
}

function applyMapTheme() {
  if (sondeMap) sondeMap.setStyle(mapStyle());
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
}

/* Utility di formattazione e piccoli helper DOM. */

const $ = id => document.getElementById(id);

const DASH = '\u2014';

function fmt(v, decimals) {
  if (v === null || v === undefined || v === '') return DASH;
  const n = Number(v);
  if (Number.isNaN(n)) return DASH;
  return n.toFixed(decimals === undefined ? 1 : decimals);
}

function fmtInt(v) {
  if (v === null || v === undefined) return DASH;
  return String(v);
}

function toDate(ts) {
  return new Date(Number(ts) * 1000);
}

function fmtTime(ts) {
  return toDate(ts).toLocaleString('it-IT', {
    day: '2-digit', month: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit'
  });
}

function fmtTimeShort(ts) {
  return toDate(ts).toLocaleTimeString('it-IT', {
    hour: '2-digit', minute: '2-digit', second: '2-digit'
  });
}

function escapeHtml(s) {
  return String(s === null || s === undefined ? '' : s)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

/* Precedenza MIMOSA: virtual > bme280 > dht22 > pms. */
function valueWithPrecedence(d, sources) {
  if (!d) return null;
  for (const s of sources) {
    if (d[s] !== null && d[s] !== undefined) return d[s];
  }
  return null;
}

const TEMP_SOURCES = ['temp_virtual', 'temp_bme280', 'temp_dht22', 'temp_pms'];
const HUM_SOURCES = ['hum_virtual', 'hum_bme280', 'hum_dht22', 'hum_pms'];
const PRESS_SOURCES = ['press_virtual', 'press_bme280'];

function temperature(d) { return valueWithPrecedence(d, TEMP_SOURCES); }
function humidity(d) { return valueWithPrecedence(d, HUM_SOURCES); }

function pressureHpa(d) {
  const p = valueWithPrecedence(d, PRESS_SOURCES);
  return p === null ? null : p / 100;
}

function mainMetric(d) {
  if (!d) return { label: 'Valore', value: DASH, unit: '' };
  if (d.water_level_cm !== null && d.water_level_cm !== undefined) {
    return { label: 'Livello', value: fmt(d.water_level_cm, 1), unit: 'cm' };
  }
  if (d.pm25 !== null && d.pm25 !== undefined) {
    return { label: 'PM2.5', value: fmt(d.pm25, 0), unit: 'µg/m³' };
  }
  return { label: 'Temp', value: fmt(temperature(d), 1), unit: '°C' };
}

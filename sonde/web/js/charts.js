/* Grafici Chart.js. */

let chartLevel, chartPM, chartTemp, chartHum, chartPress;
const allCharts = [];

function themeColors() {
  const s = getComputedStyle(document.documentElement);
  const v = (n, fb) => (s.getPropertyValue(n) || '').trim() || fb;
  return {
    tick: v('--chart-tick', '#5d6f80'),
    grid: v('--chart-grid', 'rgba(42,63,85,0.15)'),
    tipBg: v('--chart-tooltip-bg', '#ffffff'),
    tipText: v('--chart-tooltip-text', '#16232f'),
    tipBorder: v('--chart-tooltip-border', '#d5dee8')
  };
}

function baseOptions(yLabel) {
  const c = themeColors();
  return {
    responsive: true,
    maintainAspectRatio: false,
    animation: { duration: 200 },
    interaction: { mode: 'index', intersect: false },
    plugins: {
      legend: { display: true, labels: { color: c.tick, boxWidth: 12, padding: 8, font: { size: 10 } } },
      tooltip: {
        backgroundColor: c.tipBg, titleColor: c.tipText, bodyColor: c.tipText,
        borderColor: c.tipBorder, borderWidth: 1
      }
    },
    scales: {
      x: { ticks: { color: c.tick, maxTicksLimit: 10, font: { size: 10 } }, grid: { color: c.grid } },
      y: { beginAtZero: true, ticks: { color: c.tick, font: { size: 10 } }, grid: { color: c.grid }, title: { display: !!yLabel, text: yLabel, color: c.tick } }
    }
  };
}

function line(label, color, dash) {
  return {
    label, data: [], borderColor: color, backgroundColor: color + '22',
    borderWidth: dash ? 3 : 2, borderDash: dash ? [5, 3] : [],
    pointRadius: 1, fill: !dash, tension: 0.3
  };
}

function createChart(canvasId, datasets, yLabel) {
  const canvas = $(canvasId);
  if (!canvas || typeof Chart === 'undefined') return null;
  return new Chart(canvas.getContext('2d'), {
    type: 'line',
    data: { labels: [], datasets },
    options: baseOptions(yLabel)
  });
}

function initCharts() {
  chartLevel = createChart('chartLevel', [line('Livello', '#2f7fd6'), line('Ultrasuoni', '#5dade2', true)], 'cm');
  chartPM = createChart('chartPM', [line('PM1', '#e74c3c'), line('PM2.5', '#f39c12'), line('PM10', '#9b59b6')], 'µg/m³');
  chartTemp = createChart('chartTemp', [
    line('PMS', '#e74c3c'), line('DHT22', '#f39c12'),
    line('BME280', '#2ecc71'), line('Virtual', '#8e44ad', true)
  ], '°C');
  chartHum = createChart('chartHum', [
    line('PMS', '#5dade2'), line('DHT22', '#2ecc71'),
    line('BME280', '#f39c12'), line('Virtual', '#8e44ad', true)
  ], '%');
  chartPress = createChart('chartPress', [line('BME280', '#2ecc71'), line('Virtual', '#8e44ad', true)], 'hPa');

  [chartLevel, chartPM, chartTemp, chartHum, chartPress].forEach(c => { if (c) allCharts.push(c); });
}

function setChart(chart, labels, series) {
  if (!chart) return;
  chart.data.labels = labels;
  series.forEach((data, i) => {
    if (chart.data.datasets[i]) chart.data.datasets[i].data = data;
  });
  chart.update('none');
}

function updateCharts() {
  if (!allCharts.length) return;

  const enabled = selectedDevice !== ALL;
  document.querySelectorAll('.chart-container').forEach(c => c.classList.toggle('disabled', !enabled));

  const pts = enabled ? readings.slice(-50) : [];
  const labels = pts.map(r => fmtTimeShort(r.ts));

  setChart(chartLevel, labels, [
    pts.map(r => r.water_level_cm),
    pts.map(r => r.ultrasonic_cm)
  ]);
  setChart(chartPM, labels, [
    pts.map(r => r.pm1), pts.map(r => r.pm25), pts.map(r => r.pm10)
  ]);
  setChart(chartTemp, labels, [
    pts.map(r => r.temp_pms), pts.map(r => r.temp_dht22),
    pts.map(r => r.temp_bme280), pts.map(r => r.temp_virtual)
  ]);
  setChart(chartHum, labels, [
    pts.map(r => r.hum_pms), pts.map(r => r.hum_dht22),
    pts.map(r => r.hum_bme280), pts.map(r => r.hum_virtual)
  ]);
  setChart(chartPress, labels, [
    pts.map(r => r.press_bme280 === null || r.press_bme280 === undefined ? null : r.press_bme280 / 100),
    pts.map(r => r.press_virtual === null || r.press_virtual === undefined ? null : r.press_virtual / 100)
  ]);
}

function applyChartsTheme() {
  const c = themeColors();
  allCharts.forEach(ch => {
    const p = ch.options.plugins;
    p.tooltip.backgroundColor = c.tipBg;
    p.tooltip.titleColor = c.tipText;
    p.tooltip.bodyColor = c.tipText;
    p.tooltip.borderColor = c.tipBorder;
    if (p.legend && p.legend.labels) p.legend.labels.color = c.tick;
    ['x', 'y'].forEach(ax => {
      const a = ch.options.scales[ax];
      if (a.ticks) a.ticks.color = c.tick;
      if (a.grid) a.grid.color = c.grid;
      if (a.title) a.title.color = c.tick;
    });
    ch.update('none');
  });
}

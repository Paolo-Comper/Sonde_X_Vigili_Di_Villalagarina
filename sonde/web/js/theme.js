/* Toggle tema chiaro/scuro. */
(function () {
  const KEY = 'sonde-theme';

  function current() {
    return document.documentElement.getAttribute('data-theme') === 'dark' ? 'dark' : 'light';
  }

  function updateLabel() {
    const btn = $('themeToggle');
    if (!btn) return;
    const dark = current() === 'dark';
    btn.textContent = dark ? 'Giorno' : 'Notte';
  }

  function apply(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    try { localStorage.setItem(KEY, theme); } catch (e) {}
    updateLabel();
    if (typeof applyChartsTheme === 'function') applyChartsTheme();
    if (typeof applyMapTheme === 'function') applyMapTheme();
  }

  function toggle() {
    apply(current() === 'dark' ? 'light' : 'dark');
  }

  function init() {
    const btn = $('themeToggle');
    if (btn) btn.addEventListener('click', toggle);
    updateLabel();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();

// Diagramm-Logik für die Übersicht (MIGRATIONSPLAN.md Abschnitt 6a).
//
// Bewusst dünn gehalten: Aggregation, Filterung und die Ziel-URLs für den
// Drill-down kommen bereits fertig vom Server (/api/diagramme/*) — dieses
// Skript ordnet die JSON-Antworten nur Chart.js-Konfigurationen zu und
// reicht Klicks als Navigation weiter (AGENTS.md-Nachtrag: Diagrammlogik
// gehört nach Go, nicht nach charts.js).
(function () {
  "use strict";

  // Matrix-Controller/-Element und den Zoom-Plugin global registrieren —
  // beide UMD-Bündel greifen beim Laden per <script> auf das globale
  // "Chart" zu (siehe vendor/-Dateien), müssen deshalb nach chart.umd.min.js
  // eingebunden sein (siehe layout.html-Reihenfolge).
  var matrixModule = window["chartjs-chart-matrix"];
  if (matrixModule) {
    Chart.register(matrixModule.MatrixController, matrixModule.MatrixElement);
  }
  if (window.ChartZoom) {
    Chart.register(window.ChartZoom);
  }

  function cssVar(name) {
    return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  }

  function hexToRgb(hex) {
    var m = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex);
    return m ? { r: parseInt(m[1], 16), g: parseInt(m[2], 16), b: parseInt(m[3], 16) } : { r: 0, g: 0, b: 0 };
  }

  // passRateColor interpoliert zwischen der kritischen und der guten
  // Statusfarbe — dieselbe Herleitung wie internal/infra/charts.passRateColor
  // (Go), hier fürs Browser-Rendering nachgebildet.
  function passRateColor(rate) {
    var good = hexToRgb(cssVar("--status-good"));
    var bad = hexToRgb(cssVar("--status-critical"));
    var t = Math.max(0, Math.min(1, rate));
    var r = Math.round(bad.r + t * (good.r - bad.r));
    var g = Math.round(bad.g + t * (good.g - bad.g));
    var b = Math.round(bad.b + t * (good.b - bad.b));
    return "rgb(" + r + "," + g + "," + b + ")";
  }

  function noDataColor() {
    return cssVar("--gridline");
  }

  function navigateTo(url) {
    if (url) {
      window.location.assign(url);
    }
  }

  function fetchJSON(url) {
    return fetch(url, { headers: { Accept: "application/json" } }).then(function (res) {
      if (!res.ok) {
        throw new Error("Anfrage an " + url + " fehlgeschlagen: " + res.status);
      }
      return res.json();
    });
  }

  // apiURL hängt den aktuellen Filter (Zeitraum/Domain) an einen
  // /api/diagramme/*-Pfad an — dieselbe Auswahl, mit der die Seite selbst
  // gerade gerendert wurde (window.location.search enthält "zeitraum"
  // und "domain", siehe dashboard.html-Filterleiste), damit ein Diagramm
  // nicht versehentlich einen anderen Zeitraum zeigt als die
  // Kennzahlen-Kacheln darüber.
  function apiURL(path) {
    return path + window.location.search;
  }

  // --- Nachrichtenvolumen pro Tag -----------------------------------------

  function renderDailyVolume() {
    var canvas = document.getElementById("chart-verlauf");
    if (!canvas) {
      return;
    }

    fetchJSON(apiURL("/api/diagramme/verlauf"))
      .then(function (data) {
        var days = data.days || [];
        var labels = days.map(function (d) { return d.label; });
        var passData = days.map(function (d) { return d.pass; });
        var failData = days.map(function (d) { return d.fail; });
        var urls = days.map(function (d) { return d.url; });

        var chart = new Chart(canvas, {
          type: "bar",
          data: {
            labels: labels,
            datasets: [
              {
                label: "Bestanden",
                data: passData,
                backgroundColor: cssVar("--status-good"),
                stack: "volumen",
                pointURLs: urls,
              },
              {
                label: "Fehlgeschlagen",
                data: failData,
                backgroundColor: cssVar("--status-critical"),
                stack: "volumen",
                pointURLs: urls,
              },
            ],
          },
          options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
              x: { stacked: true, grid: { color: cssVar("--gridline") }, ticks: { color: cssVar("--ink-secondary") } },
              y: { stacked: true, beginAtZero: true, grid: { color: cssVar("--gridline") }, ticks: { color: cssVar("--ink-secondary") } },
            },
            plugins: {
              legend: { labels: { color: cssVar("--ink") } },
              zoom: {
                pan: { enabled: true, mode: "x" },
                zoom: { wheel: { enabled: true }, pinch: { enabled: true }, mode: "x" },
              },
            },
            onClick: function (evt, elements, chart) {
              if (!elements.length) {
                return;
              }
              var el = elements[0];
              var ds = chart.data.datasets[el.datasetIndex];
              navigateTo(ds.pointURLs && ds.pointURLs[el.index]);
            },
          },
        });

        canvas.style.height = "320px";
        renderDailyVolumeTable(days);
        return chart;
      })
      .catch(function (err) {
        console.error("Diagramm 'Nachrichtenvolumen pro Tag' konnte nicht geladen werden", err);
      });
  }

  function renderDailyVolumeTable(days) {
    var table = document.getElementById("chart-verlauf-table");
    if (!table) {
      return;
    }
    var rows = ["<caption>Nachrichtenvolumen pro Tag</caption>",
      "<tr><th>Tag</th><th>Bestanden</th><th>Fehlgeschlagen</th></tr>"];
    days.forEach(function (d) {
      rows.push(
        "<tr><td><a href=\"" + d.url + "\">" + escapeHTML(d.label) + "</a></td>" +
          "<td>" + d.pass + "</td><td>" + d.fail + "</td></tr>"
      );
    });
    table.innerHTML = rows.join("");
  }

  // --- Sendequelle × Tag (Heatmap) ----------------------------------------

  function renderHeatmap() {
    var canvas = document.getElementById("chart-heatmap");
    if (!canvas || !matrixModule) {
      return;
    }

    fetchJSON(apiURL("/api/diagramme/heatmap"))
      .then(function (data) {
        var dayLabels = data.dayLabels || [];
        var sourceLabels = data.sourceLabels || [];
        var cells = data.cells || [];

        if (dayLabels.length === 0 || sourceLabels.length === 0) {
          renderHeatmapTable(cells);
          return;
        }

        var cellWidth = 36;
        var cellHeight = 28;
        canvas.style.minWidth = Math.max(300, dayLabels.length * cellWidth + 140) + "px";
        canvas.style.height = sourceLabels.length * cellHeight + 60 + "px";

        var chart = new Chart(canvas, {
          type: "matrix",
          data: {
            datasets: [
              {
                label: "Pass-Rate",
                data: cells,
                backgroundColor: function (context) {
                  var raw = context.dataset.data[context.dataIndex];
                  return raw && raw.hasData ? passRateColor(raw.passRate) : noDataColor();
                },
                borderColor: cssVar("--surface"),
                borderWidth: 2,
                width: function (context) {
                  var area = context.chart.chartArea;
                  return area ? area.width / dayLabels.length - 2 : cellWidth;
                },
                height: function (context) {
                  var area = context.chart.chartArea;
                  return area ? area.height / sourceLabels.length - 2 : cellHeight;
                },
              },
            ],
          },
          options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
              x: {
                type: "category",
                labels: dayLabels,
                offset: true,
                ticks: { color: cssVar("--ink-secondary") },
                grid: { display: false },
              },
              y: {
                type: "category",
                labels: sourceLabels,
                offset: true,
                ticks: { color: cssVar("--ink-secondary") },
                grid: { display: false },
              },
            },
            plugins: {
              legend: { display: false },
              tooltip: {
                callbacks: {
                  title: function () {
                    return "";
                  },
                  label: function (context) {
                    var raw = context.dataset.data[context.dataIndex];
                    if (!raw.hasData) {
                      return [raw.y, raw.x, "keine Daten"];
                    }
                    return [raw.y, raw.x, "Pass-Rate: " + (raw.passRate * 100).toFixed(1) + " %", "Nachrichten: " + raw.total];
                  },
                },
              },
            },
            onClick: function (evt, elements, chart) {
              if (!elements.length) {
                return;
              }
              var el = elements[0];
              var raw = chart.data.datasets[el.datasetIndex].data[el.index];
              navigateTo(raw && raw.url);
            },
          },
        });

        renderHeatmapTable(cells);
        return chart;
      })
      .catch(function (err) {
        console.error("Diagramm 'Sendequelle × Tag' konnte nicht geladen werden", err);
      });
  }

  function renderHeatmapTable(cells) {
    var table = document.getElementById("chart-heatmap-table");
    if (!table) {
      return;
    }
    var rows = ["<caption>Sendequelle × Tag</caption>",
      "<tr><th>Quelle</th><th>Tag</th><th>Pass-Rate</th><th>Nachrichten</th></tr>"];
    cells.forEach(function (c) {
      var passRate = c.hasData ? (c.passRate * 100).toFixed(1) + " %" : "keine Daten";
      var count = c.hasData ? c.total : "–";
      rows.push(
        "<tr><td>" + escapeHTML(c.y) + "</td><td><a href=\"" + c.url + "\">" + escapeHTML(c.x) + "</a></td>" +
          "<td>" + passRate + "</td><td>" + count + "</td></tr>"
      );
    });
    table.innerHTML = rows.join("");
  }

  function escapeHTML(s) {
    var div = document.createElement("div");
    div.textContent = String(s == null ? "" : s);
    return div.innerHTML;
  }

  // --- Top-Sendequellen (horizontales Balkendiagramm) ---------------------

  function renderTopSources() {
    var canvas = document.getElementById("chart-quellen");
    if (!canvas) {
      return;
    }

    fetchJSON(apiURL("/api/diagramme/quellen"))
      .then(function (data) {
        var sources = data.sources || [];
        var labels = sources.map(function (s) { return s.label; });
        var totals = sources.map(function (s) { return s.total; });
        var colors = sources.map(function (s) { return passRateColor(s.passRate); });
        var urls = sources.map(function (s) { return s.url; });

        canvas.style.height = Math.max(120, sources.length * 28 + 60) + "px";

        var chart = new Chart(canvas, {
          type: "bar",
          data: {
            labels: labels,
            datasets: [
              {
                label: "Nachrichten",
                data: totals,
                backgroundColor: colors,
                pointURLs: urls,
              },
            ],
          },
          options: {
            indexAxis: "y",
            responsive: true,
            maintainAspectRatio: false,
            scales: {
              x: { beginAtZero: true, grid: { color: cssVar("--gridline") }, ticks: { color: cssVar("--ink-secondary") } },
              y: { grid: { display: false }, ticks: { color: cssVar("--ink-secondary") } },
            },
            plugins: {
              legend: { display: false },
              tooltip: {
                callbacks: {
                  label: function (context) {
                    var s = sources[context.dataIndex];
                    return ["Nachrichten: " + s.total, "Pass-Rate: " + (s.passRate * 100).toFixed(1) + " %"];
                  },
                },
              },
            },
            onClick: function (evt, elements, chart) {
              if (!elements.length) {
                return;
              }
              var el = elements[0];
              var ds = chart.data.datasets[el.datasetIndex];
              navigateTo(ds.pointURLs && ds.pointURLs[el.index]);
            },
          },
        });

        renderTopSourcesTable(sources);
        return chart;
      })
      .catch(function (err) {
        console.error("Diagramm 'Top-Sendequellen' konnte nicht geladen werden", err);
      });
  }

  function renderTopSourcesTable(sources) {
    var table = document.getElementById("chart-quellen-table");
    if (!table) {
      return;
    }
    var rows = ["<caption>Top-Sendequellen</caption>",
      "<tr><th>Quelle</th><th>Nachrichten</th><th>Pass-Rate</th></tr>"];
    sources.forEach(function (s) {
      rows.push(
        "<tr><td><a href=\"" + s.url + "\">" + escapeHTML(s.label) + "</a></td>" +
          "<td>" + s.total + "</td><td>" + (s.passRate * 100).toFixed(1) + " %</td></tr>"
      );
    });
    table.innerHTML = rows.join("");
  }

  // --- Verteilung nach Disposition (Donut) --------------------------------

  function dispositionColor(disposition) {
    switch (disposition) {
      case "none":
        return cssVar("--status-good");
      case "quarantine":
        return cssVar("--status-warning");
      case "reject":
        return cssVar("--status-critical");
      default:
        return cssVar("--gridline");
    }
  }

  function renderDisposition() {
    var canvas = document.getElementById("chart-disposition");
    if (!canvas) {
      return;
    }

    fetchJSON(apiURL("/api/diagramme/disposition"))
      .then(function (data) {
        var slices = (data.slices || []).filter(function (s) { return s.total > 0; });
        var labels = slices.map(function (s) { return s.label; });
        var totals = slices.map(function (s) { return s.total; });
        var colors = slices.map(function (s) { return dispositionColor(s.disposition); });
        var urls = slices.map(function (s) { return s.url; });

        canvas.style.height = "260px";

        var chart = new Chart(canvas, {
          type: "doughnut",
          data: {
            labels: labels,
            datasets: [{ data: totals, backgroundColor: colors, pointURLs: urls }],
          },
          options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
              legend: { position: "bottom", labels: { color: cssVar("--ink") } },
              tooltip: {
                callbacks: {
                  label: function (context) {
                    var total = totals.reduce(function (a, b) { return a + b; }, 0);
                    var value = context.parsed;
                    var pct = total > 0 ? ((value / total) * 100).toFixed(1) : "0.0";
                    return context.label + ": " + value + " (" + pct + " %)";
                  },
                },
              },
            },
            onClick: function (evt, elements, chart) {
              if (!elements.length) {
                return;
              }
              var el = elements[0];
              var ds = chart.data.datasets[el.datasetIndex];
              navigateTo(ds.pointURLs && ds.pointURLs[el.index]);
            },
          },
        });

        renderDispositionTable(slices);
        return chart;
      })
      .catch(function (err) {
        console.error("Diagramm 'Verteilung nach Disposition' konnte nicht geladen werden", err);
      });
  }

  function renderDispositionTable(slices) {
    var table = document.getElementById("chart-disposition-table");
    if (!table) {
      return;
    }
    var rows = ["<caption>Verteilung nach Disposition</caption>",
      "<tr><th>Disposition</th><th>Nachrichten</th></tr>"];
    slices.forEach(function (s) {
      rows.push(
        "<tr><td><a href=\"" + s.url + "\">" + escapeHTML(s.label) + "</a></td>" +
          "<td>" + s.total + "</td></tr>"
      );
    });
    table.innerHTML = rows.join("");
  }

  document.addEventListener("DOMContentLoaded", function () {
    renderDailyVolume();
    renderTopSources();
    renderDisposition();
    renderHeatmap();
  });
})();

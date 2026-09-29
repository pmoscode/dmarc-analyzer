// Chart logic for the overview (MIGRATIONSPLAN.md section 6a).
//
// Deliberately kept thin: aggregation, filtering and the drill-down
// target URLs already come ready-made from the server
// (/api/diagramme/*) — this script only maps the JSON responses to
// Chart.js configurations and forwards clicks as navigation (AGENTS.md
// addendum: chart logic belongs in Go, not in charts.js).
(function () {
  "use strict";

  // Register the matrix controller/element and the zoom plugin globally
  // — both UMD bundles access the global "Chart" when loaded via
  // <script> (see vendor/ files), so they must be included after
  // chart.umd.min.js (see the layout.html order).
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

  // passRateColor interpolates between the critical and the good status
  // color — the same derivation as internal/infra/charts.passRateColor
  // (Go), replicated here for browser rendering.
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
        throw new Error("request to " + url + " failed: " + res.status);
      }
      return res.json();
    });
  }

  // apiURL appends the current filter (period/domain) to an
  // /api/diagramme/* path — the same selection the page itself was just
  // rendered with (window.location.search contains "zeitraum" and
  // "domain", see the dashboard.html filter bar), so a chart doesn't
  // accidentally show a different period than the metric tiles above it.
  function apiURL(path) {
    return path + window.location.search;
  }

  // --- Message volume per day ---------------------------------------------

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

        renderDailyVolumeTable(days);
        registerExport("verlauf", chart, function () {
          return {
            header: ["Tag", "Bestanden", "Fehlgeschlagen"],
            rows: days.map(function (d) { return [d.label, d.pass, d.fail]; }),
          };
        });
        return chart;
      })
      .catch(function (err) {
        console.error("could not load chart 'message volume per day'", err);
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

  // --- Sending source × day (heatmap) -------------------------------------

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
        var toCSV = function () {
          return {
            header: ["Quelle", "Tag", "Pass-Rate", "Nachrichten"],
            rows: cells.map(function (c) {
              return [c.y, c.x, c.hasData ? (c.passRate * 100).toFixed(1) + " %" : "keine Daten", c.hasData ? c.total : ""];
            }),
          };
        };

        if (dayLabels.length === 0 || sourceLabels.length === 0) {
          renderHeatmapTable(cells);
          registerExport("heatmap", null, toCSV);
          return;
        }

        var cellWidth = 36;
        var cellHeight = 28;
        // Width/height depend on the number of days/sending sources. Must
        // be set on the parent wrapper, not on the canvas itself —
        // otherwise the size feedback loop documented in app.css at
        // .chart-canvas-wrap occurs. The minimum width ensures the
        // surrounding .chart-scroll actually shows a horizontal
        // scrollbar with many days.
        var wrap = document.getElementById("chart-heatmap-wrap");
        if (wrap) {
          wrap.style.minWidth = Math.max(300, dayLabels.length * cellWidth + 140) + "px";
          wrap.style.height = sourceLabels.length * cellHeight + 60 + "px";
        }

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
        registerExport("heatmap", chart, toCSV);
        return chart;
      })
      .catch(function (err) {
        console.error("could not load chart 'sending source × day'", err);
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

  // --- Chart export (MIGRATIONSPLAN.md milestone M4: "chart export as
  // PNG (browser) and CSV (table view)") -----------------------------------
  //
  // chartExports maps a chart key (the same names as the "data-chart"
  // attributes of the export buttons in dashboard.html) to access to the
  // most recently drawn Chart.js object, plus a csv() function that
  // returns exactly the rows the corresponding table view also shows —
  // no server-side image export (see MIGRATIONSPLAN.md section 6a:
  // "server-side image export dropped").
  var chartExports = {};

  function registerExport(key, chart, csvRows) {
    chartExports[key] = { chart: chart, csv: csvRows };
  }

  function downloadDataURL(filename, dataURL) {
    var a = document.createElement("a");
    a.href = dataURL;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  }

  function csvEscape(v) {
    v = String(v == null ? "" : v);
    if (/[",\n]/.test(v)) {
      v = '"' + v.replace(/"/g, '""') + '"';
    }
    return v;
  }

  function rowsToCSV(header, rows) {
    var lines = [header.map(csvEscape).join(",")];
    rows.forEach(function (row) {
      lines.push(row.map(csvEscape).join(","));
    });
    return lines.join("\r\n");
  }

  function downloadCSV(filename, header, rows) {
    var blob = new Blob([rowsToCSV(header, rows)], { type: "text/csv;charset=utf-8" });
    var url = URL.createObjectURL(blob);
    downloadDataURL(filename, url);
    URL.revokeObjectURL(url);
  }

  document.addEventListener("click", function (event) {
    var button = event.target.closest(".chart-export");
    if (!button) {
      return;
    }
    var entry = chartExports[button.dataset.chart];
    if (!entry) {
      return;
    }
    if (button.dataset.export === "png") {
      if (entry.chart) {
        downloadDataURL(button.dataset.chart + ".png", entry.chart.toBase64Image());
      }
    } else if (entry.csv) {
      var csv = entry.csv();
      downloadCSV(button.dataset.chart + ".csv", csv.header, csv.rows);
    }
  });

  // --- Top sending sources (horizontal bar chart) --------------------------

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

        // Height depends on the number of sending sources (more sources
        // -> more bar rows). Must be set on the parent wrapper, not on
        // the canvas itself — otherwise the size feedback loop
        // documented in app.css at .chart-canvas-wrap occurs.
        var wrap = document.getElementById("chart-quellen-wrap");
        if (wrap) {
          wrap.style.height = Math.max(120, sources.length * 28 + 60) + "px";
        }

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
        registerExport("quellen", chart, function () {
          return {
            header: ["Quelle", "Nachrichten", "Pass-Rate"],
            rows: sources.map(function (s) { return [s.label, s.total, (s.passRate * 100).toFixed(1) + " %"]; }),
          };
        });
        return chart;
      })
      .catch(function (err) {
        console.error("could not load chart 'top sending sources'", err);
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

  // --- Disposition breakdown (donut) ---------------------------------------

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
        registerExport("disposition", chart, function () {
          return {
            header: ["Disposition", "Nachrichten"],
            rows: slices.map(function (s) { return [s.label, s.total]; }),
          };
        });
        return chart;
      })
      .catch(function (err) {
        console.error("could not load chart 'disposition breakdown'", err);
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

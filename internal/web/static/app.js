// Sync-Fortschritt (MIGRATIONSPLAN.md Meilenstein M3: "Sync-Knopf mit
// Fortschritt (SSE) und Abbruch"). Bewusst dünn — die eigentliche Logik
// (höchstens ein Lauf gleichzeitig, Abbruch, Aufsummieren über mehrere
// Konten) steckt serverseitig in internal/app/syncjob, dieses Skript
// zeigt nur den zuletzt vom Server gemeldeten Zustand an und reicht
// Start/Abbruch als ganz normale Formular-POSTs weiter (kein eigener
// Ajax-Aufruf nötig — /abgleich und /abgleich/abbrechen leiten nach dem
// POST auf die aktuelle Seite zurück).
(function () {
  "use strict";

  var statusEl = document.getElementById("sync-status");
  var startForm = document.getElementById("sync-start-form");
  var cancelForm = document.getElementById("sync-cancel-form");
  if (!statusEl || !startForm || !cancelForm || !window.EventSource) {
    return;
  }

  function progressText(p) {
    return p.processed + " verarbeitet, " + p.new + " neu, " + p.skipped + " übersprungen, " + p.failed + " fehlerhaft";
  }

  function totalText(t) {
    return t.new + " neu, " + t.skipped + " übersprungen, " + t.failed + " fehlerhaft";
  }

  function render(state) {
    if (!state) {
      return;
    }

    if (state.status === "running") {
      startForm.hidden = true;
      cancelForm.hidden = false;
      var account = state.currentAccount ? " (" + state.currentAccount + ")" : "";
      statusEl.textContent = "Abgleich läuft" + account + ": " + progressText(state.progress);
      return;
    }

    startForm.hidden = false;
    cancelForm.hidden = true;

    switch (state.status) {
      case "done":
        statusEl.textContent = "Letzter Abgleich: " + totalText(state.total);
        break;
      case "cancelled":
        statusEl.textContent = "Abgleich abgebrochen.";
        break;
      case "failed":
        statusEl.textContent = "Abgleich fehlgeschlagen" + (state.err ? ": " + state.err : ".");
        break;
      default:
        statusEl.textContent = "";
    }
  }

  var source = new EventSource("/ereignisse");
  source.onmessage = function (event) {
    try {
      render(JSON.parse(event.data));
    } catch (err) {
      console.error("Sync-Ereignis konnte nicht gelesen werden", err);
    }
  };
})();

// Bestätigungsabfrage vor destruktiven Formularen (z. B. Konto löschen,
// settings.html) — dieselbe Sicherheitsnetz-Idee wie zuvor
// dialog.ShowConfirm in internal/ui/settings.View.confirmDelete. Ein
// generischer, delegierter Listener statt eines Inline-onsubmit-
// Attributs (von der CSP ohnehin verboten).
document.addEventListener("submit", function (event) {
  var form = event.target;
  if (form && form.dataset && form.dataset.confirm && !window.confirm(form.dataset.confirm)) {
    event.preventDefault();
  }
});


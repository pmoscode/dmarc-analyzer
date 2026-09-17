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

  // notifyResult zeigt eine Browser-Benachrichtigung für ein beendetes
  // Sync-Ergebnis (AP 7: "Browser-Benachrichtigung bei offenem Tab") — nur
  // wenn die Berechtigung bereits erteilt ist (siehe unten, Abschnitt
  // "Browser-Benachrichtigungen"). Ein serverseitig laufender Abgleich
  // läuft über die HTTP-Anfrage hinaus weiter (internal/app/syncjob) —
  // diese Benachrichtigung ist der einzige Hinweis, wenn der Tab
  // währenddessen in den Hintergrund gerückt ist.
  function notifyResult(title, body) {
    if (!("Notification" in window) || Notification.permission !== "granted") {
      return;
    }
    try {
      new Notification(title, { body: body, tag: "dmarc-analyzer-sync" });
    } catch (err) {
      console.error("Benachrichtigung konnte nicht angezeigt werden", err);
    }
  }

  // lastStatus verfolgt den zuletzt gerenderten Status, damit
  // notifyResult nur beim ÜBERGANG von "running" zu einem Endzustand
  // feuert — nicht bei jedem der vielen Fortschritts-Ereignisse während
  // eines laufenden Abgleichs, und nicht erneut beim ersten Ereignis nach
  // einem Seitenwechsel (das den zuletzt beendeten Lauf lediglich erneut
  // meldet, siehe Runner.Subscribe-Dokumentation).
  var lastStatus = null;

  function render(state) {
    if (!state) {
      return;
    }

    var wasRunning = lastStatus === "running";
    lastStatus = state.status;

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
        if (wasRunning) {
          notifyResult("Abgleich abgeschlossen", totalText(state.total));
        }
        break;
      case "cancelled":
        statusEl.textContent = "Abgleich abgebrochen.";
        if (wasRunning) {
          notifyResult("Abgleich abgebrochen", "");
        }
        break;
      case "failed":
        statusEl.textContent = "Abgleich fehlgeschlagen" + (state.err ? ": " + state.err : ".");
        if (wasRunning) {
          notifyResult("Abgleich fehlgeschlagen", state.err || "");
        }
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

// Browser-Benachrichtigungen aktivieren (AP 7) — der Knopf sitzt auf
// settings.html, existiert also nur dort; die eigentliche Anzeige einer
// Benachrichtigung (notifyResult oben) läuft seitenunabhängig, sobald die
// Berechtigung einmal erteilt ist (der Browser merkt sie sich dauerhaft,
// keine eigene Ablage nötig). Berechtigungen werden bewusst nur auf einen
// Klick hin angefragt (Nutzergeste), nie automatisch beim Laden — die
// meisten Browser blenden einen unaufgeforderten Prompt ohnehin aus oder
// verweigern ihn.
(function () {
  "use strict";

  var button = document.getElementById("notifications-enable-btn");
  var status = document.getElementById("notifications-status");
  if (!button || !status || !("Notification" in window)) {
    return;
  }

  function render() {
    switch (Notification.permission) {
      case "granted":
        button.hidden = true;
        status.textContent = "Browser-Benachrichtigungen sind aktiviert.";
        break;
      case "denied":
        button.hidden = true;
        status.textContent = "Browser-Benachrichtigungen wurden blockiert — Berechtigung in den Browser-Einstellungen dieser Seite ändern.";
        break;
      default:
        button.hidden = false;
        status.textContent = "";
    }
  }

  button.addEventListener("click", function () {
    Notification.requestPermission().then(render);
  });

  render();
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

// Datei-Import: Drag & Drop (MIGRATIONSPLAN.md Meilenstein M4). Die
// eigentliche Dateiübernahme funktioniert bereits nativ (Browser lassen
// Dateien direkt auf ein <input type="file"> fallen) — dieses Skript
// sorgt nur für optisches Feedback (Rahmen hervorheben, sobald über der
// gesamten Fläche gezogen wird, nicht nur über dem oft winzigen
// <input>) und zeigt die ausgewählten Dateinamen an, egal ob per Klick
// oder per Drop ausgewählt.
(function () {
  "use strict";

  var dropzone = document.getElementById("import-dropzone");
  var fileInput = document.getElementById("import-file-input");
  var text = document.getElementById("import-dropzone-text");
  if (!dropzone || !fileInput) {
    return;
  }

  function updateLabel() {
    if (!text) {
      return;
    }
    var files = fileInput.files;
    if (!files || files.length === 0) {
      text.textContent = "Dateien hierher ziehen oder klicken zum Auswählen";
      return;
    }
    var names = [];
    for (var i = 0; i < files.length; i++) {
      names.push(files[i].name);
    }
    text.textContent = names.join(", ");
  }

  ["dragenter", "dragover"].forEach(function (evt) {
    dropzone.addEventListener(evt, function (e) {
      e.preventDefault();
      dropzone.classList.add("dropzone-active");
    });
  });
  ["dragleave", "drop"].forEach(function (evt) {
    dropzone.addEventListener(evt, function (e) {
      e.preventDefault();
      dropzone.classList.remove("dropzone-active");
    });
  });
  dropzone.addEventListener("drop", function (e) {
    if (e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files.length) {
      fileInput.files = e.dataTransfer.files;
      updateLabel();
    }
  });
  fileInput.addEventListener("change", updateLabel);
})();


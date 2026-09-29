// Sync progress (MIGRATIONSPLAN.md milestone M3: "sync button with
// progress (SSE) and cancel"). Deliberately thin — the actual logic (at
// most one run at a time, cancel, summing up across several accounts)
// lives server-side in internal/app/syncjob; this script only displays
// the state last reported by the server and forwards start/cancel as
// plain form POSTs (no dedicated Ajax call needed — /abgleich and
// /abgleich/abbrechen redirect back to the current page after the POST).
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

  // notifyResult shows a browser notification for a finished sync result
  // (AP 7: "browser notification with an open tab") — only if permission
  // has already been granted (see below, "browser notifications"
  // section). A sync running server-side keeps going beyond the HTTP
  // request (internal/app/syncjob) — this notification is the only hint
  // if the tab was pushed into the background in the meantime.
  function notifyResult(title, body) {
    if (!("Notification" in window) || Notification.permission !== "granted") {
      return;
    }
    try {
      new Notification(title, { body: body, tag: "dmarc-analyzer-sync" });
    } catch (err) {
      console.error("could not show notification", err);
    }
  }

  // lastStatus tracks the last rendered status, so notifyResult only
  // fires on the TRANSITION from "running" to a final state — not on
  // every one of the many progress events during a running sync, and not
  // again on the first event after a page change (which just reports the
  // most recently finished run again, see the Runner.Subscribe
  // documentation).
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
      console.error("could not read sync event", err);
    }
  };
})();

// Enable browser notifications (AP 7) — the button sits on
// settings.html, so it only exists there; the actual display of a
// notification (notifyResult above) runs independently of the page, once
// permission has been granted (the browser remembers it permanently, no
// separate storage needed). Permission is deliberately only requested on
// a click (a user gesture), never automatically on load — most browsers
// suppress or deny an unsolicited prompt anyway.
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

// Confirmation prompt before destructive forms (e.g. delete account,
// settings.html) — the same safety-net idea as formerly
// dialog.ShowConfirm in internal/ui/settings.View.confirmDelete. A
// generic, delegated listener instead of an inline onsubmit attribute
// (forbidden by the CSP anyway).
document.addEventListener("submit", function (event) {
  var form = event.target;
  if (form && form.dataset && form.dataset.confirm && !window.confirm(form.dataset.confirm)) {
    event.preventDefault();
  }
});

// File import: drag & drop (MIGRATIONSPLAN.md milestone M4). Taking over
// the files themselves already works natively (browsers let files be
// dropped directly onto an <input type="file">) — this script only
// provides visual feedback (highlight the border as soon as something is
// dragged over the whole area, not just over the often tiny <input>) and
// shows the selected file names, whether chosen by click or by drop.
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

// Record detail dialogs (failed_records.html, report_detail.html): each
// row's "Details" button opens its own <dialog> (data-dialog-target
// points at its id) instead of expanding inline in the last table column
// — a long DKIM/SPF/reason list no longer distorts the row height. A
// delegated listener instead of binding per button, so rows appended
// later via htmx ("Weitere laden" on failed_records.html) work without
// any extra wiring. The close button is a native `<form method="dialog">`
// submit — no JS needed for that half. Escape-to-close and focus
// trapping are native <dialog> behavior via showModal() too.
//
// The "?" glossary hints INSIDE such a dialog are a special case, unlike
// every other .glossary-hint in the app (dashboard tiles, table headers):
// those show a plain CSS hover/focus tooltip (app.css), which works fine
// because the whole term stays in view. Inside a dialog's own scrollable
// column that assumption breaks — a tooltip that opens below the
// currently visible area needs the user to scroll down to read it, but
// scrolling moves the button out from under a mouse that hasn't itself
// moved, which un-hovers it and closes the tooltip again. So these
// buttons instead open/update a persistent panel that's part of the
// dialog's own layout (record-detail-glossary-panel in app.css) and
// stays open regardless of scroll position, until explicitly closed.
document.addEventListener("click", function (event) {
  var trigger = event.target.closest("[data-dialog-target]");
  if (trigger) {
    var dialog = document.getElementById(trigger.getAttribute("data-dialog-target"));
    if (dialog && typeof dialog.showModal === "function") {
      dialog.showModal();
    }
    return;
  }

  var hint = event.target.closest(".record-detail-dialog .glossary-hint");
  if (hint) {
    var dialogEl = hint.closest(".record-detail-dialog");
    var panel = dialogEl && dialogEl.querySelector(".record-detail-glossary-panel");
    if (panel) {
      panel.querySelector(".record-detail-glossary-panel-title").textContent = hint.getAttribute("data-term") || "";
      panel.querySelector(".record-detail-glossary-text").textContent = hint.getAttribute("data-tip") || "";
      panel.hidden = false;
    }
    return;
  }

  var panelClose = event.target.closest(".record-detail-glossary-close");
  if (panelClose) {
    var openPanel = panelClose.closest(".record-detail-glossary-panel");
    if (openPanel) {
      openPanel.hidden = true;
    }
    return;
  }

  // A click that lands on the ::backdrop (outside the dialog's own box,
  // but still inside the <dialog> element in the DOM) reports the
  // <dialog> itself as event.target — used here to close on an
  // outside click, the same way native <select>/browser dialogs behave.
  if (event.target.nodeName === "DIALOG" && event.target.hasAttribute("open")) {
    event.target.close();
  }
});

// Collapse the glossary panel again whenever a record detail dialog
// closes (close button, backdrop click, or Escape — all of them end up
// firing this), so reopening the same row's dialog later starts fresh
// instead of remembering whichever term was last explained. "close"
// does NOT bubble (unlike "click" above), but the capturing phase still
// reaches a listener on document regardless — that's what the trailing
// `true` (useCapture) is for, and it's what makes this delegated the
// same way as the click listener, including for dialogs added later via
// htmx ("Weitere laden" on failed_records.html).
document.addEventListener("close", function (event) {
  var dlg = event.target;
  if (!(dlg instanceof HTMLDialogElement) || !dlg.classList.contains("record-detail-dialog")) {
    return;
  }
  var panel = dlg.querySelector(".record-detail-glossary-panel");
  if (panel) {
    panel.hidden = true;
  }
}, true);

// Remember the filter per view (dashboard, reports, domains, sources
// each have their own filter-bar form, see internal/web/filter.go —
// "filters live in the URL"). Without this script, every view forgets
// its filter when switching, because clicking the main nav always leads
// to the plain page URL without query parameters. Submitting the filter
// form remembers the resulting query string in localStorage per page
// path; a later visit to the same view without query parameters
// restores it. The "reset filter" link clears the remembered state
// before it leads back to the parameter-less URL. Deliberately no
// server-side state (not a state change in the sense of AGENTS.md) —
// pure display convenience per browser, no POST/CSRF needed.
(function () {
  "use strict";

  if (!document.querySelector("form.filter-bar")) {
    return;
  }

  var storageKey = "dmarc-analyzer:filter:" + window.location.pathname;

  try {
    if (window.location.search) {
      window.localStorage.setItem(storageKey, window.location.search);
    } else {
      var saved = window.localStorage.getItem(storageKey);
      if (saved) {
        window.location.search = saved;
      }
    }
  } catch (err) {
    // localStorage can fail in private modes or due to browser settings
    // — filters then simply work as before, without remembering state.
  }

  document.addEventListener("click", function (event) {
    if (!event.target.closest(".filter-reset")) {
      return;
    }
    try {
      window.localStorage.removeItem(storageKey);
    } catch (err) {
      // see above.
    }
  });
})();

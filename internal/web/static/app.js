// An exercise is either rep-based (e.g. bench 3x10) or time-based (e.g. plank 60s),
// never both. Keep the two inputs mutually exclusive in each entry row so the user
// can't hit the server-side CHECK constraint. `readonly` (not `disabled`) is used so
// the field is still submitted, keeping the parallel form arrays aligned.
function syncEntryRow(row) {
  const reps = row.querySelector('[name="entry_reps"]');
  const dur = row.querySelector('[name="entry_duration"]');
  if (!reps || !dur) return;
  const repsFilled = reps.value.trim() !== "";
  const durFilled = dur.value.trim() !== "";
  dur.readOnly = repsFilled;
  reps.readOnly = durFilled;
}

document.addEventListener("input", (e) => {
  const el = e.target;
  if (!el.closest) return;
  const row = el.closest(".entry-row");
  if (row) syncEntryRow(row);
});

function syncAllRows() {
  document.querySelectorAll(".entry-row").forEach(syncEntryRow);
}

document.addEventListener("DOMContentLoaded", syncAllRows);
// re-sync after HTMX swaps in a fresh entry row
document.body.addEventListener("htmx:afterSwap", syncAllRows);

// remove an exercise row (delegated so it works for HTMX-added rows; no inline JS for CSP)
document.addEventListener("click", (e) => {
  const btn = e.target.closest && e.target.closest("[data-remove-entry]");
  if (btn) {
    const row = btn.closest(".entry-row");
    if (row) row.remove();
  }
});

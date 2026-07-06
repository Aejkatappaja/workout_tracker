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

// Auth gopher: pupils drift toward the caret as the user types, and the arms rise
// to cover the eyes while a password field is focused. The arms are driven from JS
// (not a CSS transition) so the raise/lower tween can be interrupted and eased from
// its current position on rapid focus changes. Motion is skipped under
// prefers-reduced-motion.
(function () {
  const mascot = document.querySelector(".gopher-mascot");
  const form = document.querySelector(".auth-form");
  if (!mascot || !form) return;
  const eyes = [mascot.querySelector("#g-eye-l"), mascot.querySelector("#g-eye-r")];
  const armGroup = mascot.querySelector("#g-arms");
  const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  // Values are in SVG user units (viewBox is ~402 wide), not screen px.
  const MAX_X = 42, DOWN_Y = 14;
  const ARM_PARKED = 220; // below the viewBox crop -> hidden; 0 covers the eyes

  function look(x, y) {
    const t = `translate(${x} ${y})`;
    eyes.forEach((eye) => eye.setAttribute("transform", t));
  }
  function track(input) {
    if (reduce) return;
    const ratio = Math.min(input.value.length / 16, 1); // 0 empty -> 1 around 16 chars
    look(ratio * MAX_X, DOWN_Y);                         // centered when empty, drifts right as text fills
  }

  // Arm raise/lower, tweened from the current position so an interrupted tween resumes smoothly.
  let armY = ARM_PARKED, armRAF = null;
  function setArms(y) {
    armY = y;
    armGroup.setAttribute("transform", `translate(0 ${y})`);
  }
  function tweenArms(to, dur) {
    if (armRAF) cancelAnimationFrame(armRAF);
    if (reduce) { setArms(to); return; }
    const from = armY, start = performance.now();
    function frame(now) {
      const t = Math.min((now - start) / dur, 1);
      const e = 1 - Math.pow(1 - t, 3); // easeOutCubic
      setArms(from + (to - from) * e);
      if (t < 1) armRAF = requestAnimationFrame(frame);
    }
    armRAF = requestAnimationFrame(frame);
  }
  setArms(ARM_PARKED); // start hidden

  form.addEventListener("focusin", (e) => {
    const el = e.target;
    if (el.type === "password") { look(0, 0); tweenArms(0, 400); return; }
    if (el.tagName === "INPUT") { tweenArms(ARM_PARKED, 320); track(el); }
  });
  form.addEventListener("input", (e) => {
    if (e.target.tagName === "INPUT" && e.target.type !== "password") track(e.target);
  });
  form.addEventListener("focusout", (e) => {
    if (!form.contains(e.relatedTarget)) { look(0, 0); tweenArms(ARM_PARKED, 320); }
  });
})();

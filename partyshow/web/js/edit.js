"use strict";

PS.add(
  {
    e_round: "Runde", e_round_title: "Titel der Runde", e_round_intro: "Ansage vor der Runde",
    e_question: "Frage", e_correct: "richtig", e_host_line: "Spruch nach der Auflösung",
    e_seconds: "Sekunden", e_delete: "Löschen", e_up: "↑", e_down: "↓",
    e_add_q: "+ Frage", e_add_option: "+ Antwort", e_del_round: "Runde löschen",
    e_saved: "Gespeichert ✓", e_unsaved: "Ungespeicherte Änderungen", e_copied: "Kopiert ✓",
    e_invalid: "Dieser Link ist ungültig.", e_new_round: "Neue Runde", e_new_q: "Neue Frage",
    e_confirm_del_round: "Ganze Runde löschen?",
  },
  {
    e_link_t: "Save this link!",
    e_link_d: "It's your access to the show – without it, nobody (not even us) can find it again.",
    e_copy: "Copy link",
    e_site_warn: "Note: your website could not be read. The questions are based on your facts only.",
    e_title: "Show title", e_welcome: "Welcome (read by the host voice)", e_outro: "Closing words",
    e_add_round: "+ Add round",
    e_plan: "Your show is unlocked for up to", e_players: "players.",
    e_save: "Save", e_host: "Start show (projector)",
    e_round: "Round", e_round_title: "Round title", e_round_intro: "Host line before the round",
    e_question: "Question", e_correct: "correct", e_host_line: "Host line after the reveal",
    e_seconds: "seconds", e_delete: "Delete", e_up: "↑", e_down: "↓",
    e_add_q: "+ Question", e_add_option: "+ Answer", e_del_round: "Delete round",
    e_saved: "Saved ✓", e_unsaved: "Unsaved changes", e_copied: "Copied ✓",
    e_invalid: "This link is not valid.", e_new_round: "New round", e_new_q: "New question",
    e_confirm_del_round: "Delete the whole round?",
  },
);

PS.apply();
const { el, t } = PS;
const $ = (id) => document.getElementById(id);
const ref = PS.showRef();
let data = null;
let dirty = false;

function markDirty() {
  dirty = true;
  $("status").textContent = t("e_unsaved");
}

function move(arr, i, d) {
  const j = i + d;
  if (j < 0 || j >= arr.length) return;
  [arr[i], arr[j]] = [arr[j], arr[i]];
}

function renderQuestion(round, qi) {
  const q = round.questions[qi];
  const name = "c" + Math.random().toString(36).slice(2);
  const opts = q.options.map((o, oi) =>
    el("div", { class: "opt" },
      el("input", { type: "radio", name, title: t("e_correct"), checked: q.correct === oi, onchange: () => { q.correct = oi; markDirty(); } }),
      el("input", { type: "text", value: o, maxlength: 120, oninput: (e) => { q.options[oi] = e.target.value; markDirty(); } }),
      q.options.length > 2 ? el("button", { class: "btn small secondary", title: t("e_delete"), onclick: () => {
        q.options.splice(oi, 1);
        if (q.correct >= q.options.length) q.correct = 0;
        else if (q.correct > oi) q.correct--;
        markDirty(); render();
      } }, "×") : null,
    ));
  return el("div", { class: "q" },
    el("div", { class: "muted small" }, t("e_question") + " " + (qi + 1)),
    el("input", { type: "text", value: q.text, maxlength: 300, oninput: (e) => { q.text = e.target.value; markDirty(); } }),
    el("div", { class: "opts" }, opts),
    el("input", { type: "text", value: q.host_line || "", maxlength: 300, placeholder: t("e_host_line"), style: "margin-top:8px",
      oninput: (e) => { q.host_line = e.target.value; markDirty(); } }),
    el("div", { class: "tools" },
      q.options.length < 4 ? el("button", { class: "btn small secondary", onclick: () => { q.options.push(""); markDirty(); render(); } }, t("e_add_option")) : null,
      el("label", { class: "check", style: "margin:0" },
        el("input", { type: "number", min: 5, max: 90, value: q.seconds || 20, style: "width:80px",
          oninput: (e) => { q.seconds = parseInt(e.target.value, 10) || 20; markDirty(); } }),
        el("span", {}, t("e_seconds"))),
      el("span", { class: "spacer" }),
      el("button", { class: "btn small secondary", onclick: () => { move(round.questions, qi, -1); markDirty(); render(); } }, t("e_up")),
      el("button", { class: "btn small secondary", onclick: () => { move(round.questions, qi, 1); markDirty(); render(); } }, t("e_down")),
      el("button", { class: "btn small danger", onclick: () => { round.questions.splice(qi, 1); markDirty(); render(); } }, t("e_delete")),
    ),
  );
}

function render() {
  const root = $("rounds");
  root.replaceChildren();
  data.rounds.forEach((round, ri) => {
    root.append(el("div", { class: "card round" },
      el("div", { class: "round-head" },
        el("h2", { style: "margin:0" }, t("e_round") + " " + (ri + 1)),
        el("span", { class: "spacer" }),
        el("button", { class: "btn small secondary", onclick: () => { move(data.rounds, ri, -1); markDirty(); render(); } }, t("e_up")),
        el("button", { class: "btn small secondary", onclick: () => { move(data.rounds, ri, 1); markDirty(); render(); } }, t("e_down")),
        el("button", { class: "btn small danger", onclick: () => {
          if (confirm(t("e_confirm_del_round"))) { data.rounds.splice(ri, 1); markDirty(); render(); }
        } }, t("e_del_round")),
      ),
      el("label", {}, t("e_round_title")),
      el("input", { type: "text", value: round.title, maxlength: 120, oninput: (e) => { round.title = e.target.value; markDirty(); } }),
      el("label", {}, t("e_round_intro")),
      el("input", { type: "text", value: round.intro || "", maxlength: 300, oninput: (e) => { round.intro = e.target.value; markDirty(); } }),
      round.questions.map((_, qi) => renderQuestion(round, qi)),
      el("button", { class: "btn small secondary", onclick: () => {
        round.questions.push({ text: t("e_new_q"), options: ["", ""], correct: 0, seconds: 20 });
        markDirty(); render();
      } }, t("e_add_q")),
    ));
  });
}

function collect() {
  data.title = $("title").value;
  data.welcome = $("welcome").value;
  data.outro = $("outro").value;
  return data;
}

async function save() {
  $("error").textContent = "";
  try {
    const res = await PS.api("PUT", "/api/shows/" + ref.id, collect(), ref.token);
    data = res;
    dirty = false;
    $("status").textContent = t("e_saved");
    render();
    return true;
  } catch (e) {
    $("error").textContent = e.message;
    return false;
  }
}

async function load() {
  try {
    data = await PS.api("GET", "/api/shows/" + ref.id, null, ref.token);
  } catch (e) {
    $("load-error").textContent = e.status === 403 || e.status === 404 ? t("e_invalid") : e.message;
    return;
  }
  $("editor").classList.remove("hidden");
  if (new URLSearchParams(location.search).get("w") === "site") $("site-warning").classList.remove("hidden");
  $("title").value = data.title;
  $("welcome").value = data.welcome || "";
  $("outro").value = data.outro || "";
  $("max-players").textContent = data.max_players;
  ["title", "welcome", "outro"].forEach((id) => $(id).addEventListener("input", markDirty));
  render();
}

$("save").addEventListener("click", save);
$("add-round").addEventListener("click", () => {
  data.rounds.push({ title: t("e_new_round"), intro: "", questions: [{ text: t("e_new_q"), options: ["", ""], correct: 0, seconds: 20 }] });
  markDirty(); render();
});
$("host").addEventListener("click", async () => {
  if (dirty && !(await save())) return;
  // Restart so the room plays the latest saved version.
  try { await PS.api("POST", "/api/shows/" + ref.id + "/restart", null, ref.token); } catch (_) { /* host page opens it anyway */ }
  window.open("/host/" + ref.id + "#" + encodeURIComponent(ref.token), "_blank");
});
$("copy-link").addEventListener("click", async () => {
  try {
    await navigator.clipboard.writeText(location.href);
    $("copied").textContent = t("e_copied");
  } catch (_) {
    $("copied").textContent = location.href;
  }
});
window.addEventListener("beforeunload", (e) => { if (dirty) { e.preventDefault(); e.returnValue = ""; } });

load();

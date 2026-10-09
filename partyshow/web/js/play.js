"use strict";

const P = {
  de: {
    code: "Code von der Leinwand", name: "Dein Name", join: "Mitspielen", wait_start: "Du bist dabei! Gleich geht's los …",
    look: "Schau auf die Leinwand!", answered: "Antwort gespeichert – warte auf die anderen …",
    right: "Richtig!", wrong: "Leider falsch", none: "Keine Antwort", points: "Punkte", rank: "Platz",
    of: "von", round: "Runde", final: "Fertig! Danke fürs Mitspielen.", reconnecting: "Verbindung wird wiederhergestellt …",
    err_no_room: "Kein Spiel mit diesem Code. Bitte prüfe den Code auf der Leinwand.",
    err_full: "Das Spiel ist voll.", err_ended: "Dieses Spiel ist schon vorbei.", err_bad_name: "Bitte gib einen Namen ein.",
    kicked: "Du wurdest vom Spiel entfernt.", too_late: "Zu spät – die Zeit ist abgelaufen.",
  },
  en: {
    code: "Code from the screen", name: "Your name", join: "Join", wait_start: "You're in! Starting soon …",
    look: "Look at the big screen!", answered: "Answer saved – waiting for the others …",
    right: "Correct!", wrong: "Wrong", none: "No answer", points: "points", rank: "Rank",
    of: "of", round: "Round", final: "Done! Thanks for playing.", reconnecting: "Reconnecting …",
    err_no_room: "No game with this code. Please check the code on the screen.",
    err_full: "The game is full.", err_ended: "This game is already over.", err_bad_name: "Please enter a name.",
    kicked: "You were removed from the game.", too_late: "Too late – time is up.",
  },
};

const { el } = PS;
const $ = (id) => document.getElementById(id);
// append() would turn null into the text "null"; skip empty children instead.
const put = (parent, ...kids) => parent.append(...kids.flat().filter((k) => k !== null && k !== undefined && k !== false));
let L = P[PS.lang];
let code = (location.pathname.split("/").filter(Boolean)[1] || "").toUpperCase();
let ws = null;
let me = null;
let state = null;
let stopped = false;
let retry = 0;
let timerHandle = null;
let lastVibe = "";

const storeKey = () => "ps_player_" + code;
function saved() {
  try { return JSON.parse(localStorage.getItem(storeKey()) || "null"); } catch (_) { return null; }
}
function save(v) {
  try { localStorage.setItem(storeKey(), JSON.stringify(v)); } catch (_) { /* private mode */ }
}

function vibrate(ms) { if (navigator.vibrate) navigator.vibrate(ms); }

function joinForm(error) {
  stopped = true;
  let savedName = "";
  try { savedName = localStorage.getItem("ps_name") || ""; } catch (_) { /* ignore */ }
  const codeIn = el("input", { type: "text", value: code, maxlength: 5, autocapitalize: "characters", autocomplete: "off", style: "text-align:center;font-size:1.6rem;letter-spacing:.2em" });
  const nameIn = el("input", { type: "text", value: savedName, maxlength: 20, autocomplete: "nickname", style: "text-align:center;font-size:1.3rem" });
  const form = el("form", { onsubmit: (e) => {
    e.preventDefault();
    code = codeIn.value.trim().toUpperCase();
    const name = nameIn.value.trim();
    if (!name) { joinForm(L.err_bad_name); return; }
    try { localStorage.setItem("ps_name", name); } catch (_) { /* ignore */ }
    if (location.pathname !== "/play/" + code) history.replaceState(null, "", "/play/" + code);
    stopped = false;
    connect({ type: "join", name });
  } },
    el("label", {}, L.code), codeIn,
    el("label", {}, L.name), nameIn,
    el("p", { class: "error" }, error || ""),
    el("button", { class: "btn", type: "submit", style: "width:100%;margin-top:10px" }, L.join));
  $("main").replaceChildren(form);
  (code ? nameIn : codeIn).focus();
}

function message(text, cls) {
  $("main").replaceChildren(el("div", { class: cls || "pq" }, text));
}

function render() {
  const s = state;
  if (!s) return;
  L = P[s.language === "en" ? "en" : "de"];
  clearInterval(timerHandle);
  $("h-right").textContent = me ? me.name + " · " + s.score : "";
  const main = $("main");
  main.replaceChildren();

  switch (s.phase) {
    case "lobby":
      put(main, el("div", { class: "pq" }, L.wait_start), el("div", { class: "muted" }, s.title));
      break;
    case "intro":
      put(main, el("div", { class: "muted" }, L.round), el("div", { class: "result" }, s.round_title), el("div", { class: "muted" }, L.look));
      break;
    case "question": {
      if (s.answered) {
        put(main, el("div", { class: "pq" }, L.answered));
        break;
      }
      const two = s.options.length === 2;
      put(main, 
        el("div", { class: "pq" }, s.question),
        el("div", { class: "pad" + (two ? " two" : "") }, s.options.map((o, i) =>
          el("button", { class: "c" + i, onclick: (e) => answer(i, e.currentTarget) },
            el("span", { class: "shape" }, PS.SHAPES[i]), el("span", {}, o)))),
        el("div", { class: "muted", id: "left" }, ""));
      const end = Date.now() + s.ms_left;
      const tick = () => {
        const left = Math.max(0, Math.ceil((end - Date.now()) / 1000));
        const n = $("left");
        if (n) n.textContent = left + " s";
        if (left <= 0) clearInterval(timerHandle);
      };
      tick();
      timerHandle = setInterval(tick, 250);
      break;
    }
    case "reveal": {
      let text = L.none;
      let cls = "result";
      if (s.answered) {
        text = s.last_right ? L.right : L.wrong;
        cls += s.last_right ? " ok" : " bad";
      }
      put(main, 
        el("div", { class: cls }, text),
        s.last_points ? el("div", { class: "bigscore" }, "+" + s.last_points) : null,
        el("div", { class: "muted" }, s.options[s.correct] ? "✓ " + s.options[s.correct] : ""),
        el("div", {}, L.rank + " " + s.rank + " " + L.of + " " + s.players));
      if (lastVibe !== s.question) { lastVibe = s.question; vibrate(s.last_right ? 60 : [40, 60, 40]); }
      break;
    }
    case "scores":
      put(main, el("div", { class: "muted" }, L.rank), el("div", { class: "bigscore" }, s.rank + ". "),
        el("div", {}, s.score + " " + L.points), el("div", { class: "muted" }, L.look));
      break;
    case "final":
      put(main, el("div", { class: "bigscore" }, s.rank + ". " + L.rank.toLowerCase()), el("div", {}, s.score + " " + L.points),
        el("div", { class: "pq" }, L.final));
      break;
  }
}

function answer(i, btn) {
  if (!ws || ws.readyState !== WebSocket.OPEN) return;
  document.querySelectorAll(".pad button").forEach((b) => { b.disabled = true; });
  btn.classList.add("chosen");
  vibrate(30);
  ws.send(JSON.stringify({ type: "answer", option: i }));
}

function connect(hello) {
  ws = new WebSocket(PS.wsURL("/ws/play/" + encodeURIComponent(code)));
  ws.onopen = () => {
    retry = 0;
    ws.send(JSON.stringify(hello));
  };
  ws.onmessage = (ev) => {
    const m = JSON.parse(ev.data);
    switch (m.type) {
      case "joined":
        me = m;
        save({ id: m.id, token: m.token });
        render(); // the first state may arrive before this message
        break;
      case "state":
        state = m.state;
        render();
        break;
      case "kicked":
        stopped = true;
        save(null);
        message(L.kicked);
        break;
      case "error":
        if (m.code === "not_now") { message(L.too_late); break; }
        if (m.code === "answered") break;
        if (m.code === "bad_rejoin") { save(null); joinForm(); break; }
        joinForm(L["err_" + m.code] || m.message);
        break;
    }
  };
  ws.onclose = () => {
    if (stopped) return;
    message(L.reconnecting, "muted");
    retry = Math.min(retry + 1, 6);
    // After the first join, reconnect as the same player.
    const prev = saved();
    const next = prev ? { type: "rejoin", id: prev.id, token: prev.token } : hello;
    setTimeout(() => { if (!stopped) connect(next); }, 400 * retry);
  };
}

// Start: rejoin if this phone already played in this room, else show the form.
async function start() {
  if (!code) { joinForm(); return; }
  try {
    const info = await PS.api("GET", "/api/rooms/" + encodeURIComponent(code));
    L = P[info.language === "en" ? "en" : "de"];
  } catch (_) {
    joinForm(L.err_no_room);
    return;
  }
  const prev = saved();
  if (prev) connect({ type: "rejoin", id: prev.id, token: prev.token });
  else joinForm();
}

start();

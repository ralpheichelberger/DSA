"use strict";

// Texts for the big screen. The show's own language decides, not the browser's.
const H = {
  de: {
    join: "Mitspielen:", scan: "QR-Code scannen oder Code eingeben auf", players: "Mitspieler",
    waiting: "Warten auf Mitspieler …", start_hint: "Leertaste oder „Weiter“ startet die Show",
    round: "Runde", of: "von", question: "Frage", answered: "Antworten", right: "Richtig:",
    leaderboard: "Zwischenstand", final: "Die Gewinner!", reconnecting: "Verbindung wird wiederhergestellt …",
    voice_on: "🔊 Stimme an", voice_off: "🔇 Stimme aus", kick: "Mitspieler entfernen?",
    restart_q: "Show neu starten? Alle Punkte gehen verloren.", demo: "Demo: Öffne den Mitspiel-Link auf deinem Handy oder in einem zweiten Tab.",
    full: "Voll – für mehr Mitspieler bitte upgraden.", invalid: "Dieser Link ist ungültig.",
    winner: "Der Sieg geht an", points: "Punkte",
    next: "Weiter ▸", restart: "Neu starten", made: "Erstellt mit",
  },
  en: {
    join: "Join the game:", scan: "Scan the QR code or enter the code at", players: "players",
    waiting: "Waiting for players …", start_hint: "Press space or “Next” to start",
    round: "Round", of: "of", question: "Question", answered: "Answers", right: "Correct:",
    leaderboard: "Leaderboard", final: "The winners!", reconnecting: "Reconnecting …",
    voice_on: "🔊 Voice on", voice_off: "🔇 Voice off", kick: "Remove this player?",
    restart_q: "Restart the show? All points will be lost.", demo: "Demo: open the join link on your phone or in a second tab.",
    full: "Full – upgrade for more players.", invalid: "This link is not valid.",
    winner: "And the winner is", points: "points",
    next: "Next ▸", restart: "Restart", made: "Made with",
  },
};

PS.add({}, {
  h_overlay: "Connect the laptop to a projector or TV, turn on the speakers – then start.",
  h_voice: "Host voice", h_start: "Fullscreen & go", h_restart: "Restart", h_next: "Next ▸",
  brand_made: "Made with",
});
PS.apply();

const { el } = PS;
const $ = (id) => document.getElementById(id);
// append() would turn null into the text "null"; skip empty children instead.
const put = (parent, ...kids) => parent.append(...kids.flat().filter((k) => k !== null && k !== undefined && k !== false));
const ref = PS.showRef();
const isDemo = new URLSearchParams(location.search).get("demo") === "1";

let L = H[PS.lang];
let state = null;
let room = null;
let ws = null;
let voice = true;
let started = false;
let lastSpokenKey = "";
let timerHandle = null;
let retry = 0;

// ---- voice ----
function speak(text) {
  if (!voice || !started || !text || !("speechSynthesis" in window)) return;
  const u = new SpeechSynthesisUtterance(text);
  const lang = state && state.language === "en" ? "en" : "de";
  u.lang = lang === "en" ? "en-GB" : "de-DE";
  const voices = speechSynthesis.getVoices().filter((v) => v.lang.toLowerCase().startsWith(lang));
  const preferred = voices.find((v) => /natural|neural|premium|enhanced|google/i.test(v.name)) || voices[0];
  if (preferred) u.voice = preferred;
  u.rate = 1.02;
  speechSynthesis.cancel();
  speechSynthesis.speak(u);
}

function speakFor(s) {
  const q = s.question;
  const key = [s.phase, s.round_no, s.question_no].join(":");
  if (key === lastSpokenKey) return;
  lastSpokenKey = key;
  switch (s.phase) {
    case "lobby": speak(s.welcome); break;
    case "intro": speak(L.round + " " + s.round_no + ": " + s.round_title + ". " + (s.round_intro || "")); break;
    case "question": speak(q.text); break;
    case "reveal": speak(L.right + " " + q.options[q.correct] + ". " + (q.host_line || "")); break;
    case "scores": {
      const top = s.players[0];
      if (top) speak(L.leaderboard + ": " + top.name + " " + top.score + " " + L.points + ".");
      break;
    }
    case "final": {
      const top = s.players[0];
      speak((top ? L.winner + " " + top.name + "! " : "") + (s.outro || ""));
      break;
    }
  }
}

// The screen speaks the show's language, whatever the laptop's browser language is.
function updateControls() {
  $("voice-toggle").textContent = voice ? L.voice_on : L.voice_off;
  $("next").textContent = L.next;
  $("restart").textContent = L.restart;
  document.querySelector(".brand span").textContent = L.made;
}

// ---- rendering ----
function answersGrid(q, reveal) {
  return el("div", { class: "answers" }, q.options.map((o, i) => {
    const cls = ["answer", "c" + i];
    if (reveal) cls.push(i === q.correct ? "right" : "dim");
    return el("div", { class: cls.join(" ") },
      el("span", { class: "shape" }, PS.SHAPES[i]),
      el("span", {}, o),
      reveal ? el("span", { class: "count" }, String(q.counts[i])) : null);
  }));
}

function board(players, n) {
  return el("div", { class: "board" }, players.slice(0, n).map((p) =>
    el("div", { class: "line" },
      el("span", { class: "rank" }, p.rank + "."),
      el("span", {}, p.name),
      el("span", { class: "plus" }, p.last_points ? "+" + p.last_points : ""),
      el("span", { class: "pts" }, String(p.score)))));
}

function kick(p) {
  if (confirm(L.kick + "\n" + p.name)) send({ type: "kick", id: p.id });
}

function startTimer(q) {
  clearInterval(timerHandle);
  const end = Date.now() + q.ms_left;
  const tick = () => {
    const left = Math.max(0, end - Date.now());
    const tEl = $("timer");
    const bar = $("bar");
    if (tEl) tEl.textContent = String(Math.ceil(left / 1000));
    if (bar) bar.style.width = (100 * left / (q.seconds * 1000)) + "%";
    if (left <= 0) clearInterval(timerHandle);
  };
  tick();
  timerHandle = setInterval(tick, 200);
}

function render() {
  const s = state;
  if (!s) return;
  L = H[s.language === "en" ? "en" : "de"];
  updateControls();
  clearInterval(timerHandle);
  const main = $("main");
  main.replaceChildren();
  $("top-left").textContent = s.title;
  $("top-right").textContent = s.phase === "lobby" || s.phase === "final" ? s.player_count + " " + L.players
    : L.round + " " + s.round_no + " " + L.of + " " + s.round_count;

  switch (s.phase) {
    case "lobby": {
      const joinURL = room ? room.join_url : "";
      put(main, 
        el("div", { class: "lobby" },
          room ? el("img", { src: "/qr/" + room.code + ".png", alt: "QR" }) : el("div"),
          el("div", {},
            el("div", { class: "big-title" }, L.join),
            el("div", { class: "muted", style: "font-size:1.8vw" }, L.scan),
            el("div", { class: "join-url" }, joinURL.replace(/^https?:\/\//, "").replace(/\/play\/.*/, "/play")),
            el("div", { class: "join-code" }, room ? room.code : ""),
            el("div", { class: "muted", style: "font-size:1.8vw" },
              s.player_count + " / " + s.max_players + " " + L.players +
              (s.player_count >= s.max_players ? " – " + L.full : "")),
          )),
        s.players.length ? el("div", { class: "names" }, s.players.map((p) => el("span", { onclick: () => kick(p) }, p.name)))
          : el("p", { class: "muted", style: "font-size:1.8vw;margin-top:3vh" }, L.waiting),
        el("p", { class: "muted", style: "font-size:1.4vw" }, L.start_hint),
        isDemo ? el("p", { class: "notice", style: "font-size:1.4vw" }, L.demo + " " + joinURL) : null,
      );
      break;
    }
    case "intro":
      put(main, 
        el("div", { class: "muted", style: "font-size:2.4vw" }, L.round + " " + s.round_no + " " + L.of + " " + s.round_count),
        el("div", { class: "huge" }, s.round_title),
        el("div", { class: "host-line" }, s.round_intro || ""));
      break;
    case "question": {
      const q = s.question;
      put(main, 
        el("div", { class: "row", style: "width:90vw;font-size:1.8vw;color:var(--muted)" },
          el("span", {}, L.question + " " + s.question_no + "/" + s.questions_in_round),
          el("span", { class: "spacer" }),
          el("span", {}, L.answered + ": " + s.answered + "/" + s.player_count),
          el("span", { class: "timer", id: "timer" }, "")),
        el("div", { class: "question-text" }, q.text),
        answersGrid(q, false),
        el("div", { class: "timer-bar" }, el("div", { id: "bar" })));
      startTimer(q);
      break;
    }
    case "reveal": {
      const q = s.question;
      put(main, 
        el("div", { class: "question-text" }, q.text),
        answersGrid(q, true),
        q.host_line ? el("div", { class: "host-line" }, q.host_line) : null,
        q.explanation ? el("div", { class: "explanation" }, q.explanation) : null);
      break;
    }
    case "scores":
      put(main, el("div", { class: "big-title" }, L.leaderboard), board(s.players, 8));
      break;
    case "final": {
      const [a, b, c] = s.players;
      const step = (p, cls) => p ? el("div", { class: "step " + cls },
        el("div", { class: "name" }, p.name), el("div", { class: "pts" }, p.score + " " + L.points)) : el("div", { class: "step " + cls });
      put(main, 
        el("div", { class: "huge" }, L.final),
        el("div", { class: "podium" }, step(b, "p2"), step(a, "p1"), step(c, "p3")),
        s.outro ? el("div", { class: "host-line" }, s.outro) : null);
      break;
    }
  }
  $("next").classList.toggle("hidden", s.phase === "final");
  speakFor(s);
}

// ---- connection ----
function send(msg) {
  if (ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify(msg));
}

function connect() {
  ws = new WebSocket(PS.wsURL("/ws/host/" + ref.id));
  ws.onopen = () => {
    retry = 0;
    $("conn").textContent = "";
    ws.send(JSON.stringify({ type: "auth", token: ref.token }));
  };
  ws.onmessage = (ev) => {
    const m = JSON.parse(ev.data);
    if (m.type === "room") { room = m; render(); }
    else if (m.type === "state") { state = m.state; render(); }
    else if (m.type === "error") { $("fatal").textContent = L.invalid; $("overlay").classList.remove("hidden"); }
  };
  ws.onclose = () => {
    if ($("fatal").textContent) return;
    $("conn").textContent = L.reconnecting;
    retry = Math.min(retry + 1, 6);
    setTimeout(connect, 500 * retry);
  };
}

// ---- controls ----
$("start").addEventListener("click", () => {
  voice = $("voice-on").checked;
  started = true;
  lastSpokenKey = "";
  $("overlay").classList.add("hidden");
  if (document.documentElement.requestFullscreen) document.documentElement.requestFullscreen().catch(() => {});
  render();
});
$("next").addEventListener("click", () => send({ type: "next" }));
$("voice-toggle").addEventListener("click", () => {
  voice = !voice;
  if (!voice && "speechSynthesis" in window) speechSynthesis.cancel();
  updateControls();
});
$("restart").addEventListener("click", async () => {
  if (!confirm(L.restart_q)) return;
  try {
    await PS.api("POST", "/api/shows/" + ref.id + "/restart?force=1", null, ref.token);
  } catch (_) { /* reconnect shows the error */ }
  room = null;
  state = null;
  lastSpokenKey = "";
  if (ws) ws.close();
});
document.addEventListener("keydown", (e) => {
  if (!started) return;
  if (e.code === "Space" || e.code === "ArrowRight" || e.code === "PageDown") {
    e.preventDefault();
    send({ type: "next" });
  }
});
if ("speechSynthesis" in window) speechSynthesis.onvoiceschanged = () => {};

updateControls();
connect();

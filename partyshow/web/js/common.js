// Shared helpers: language, translations, DOM building and API calls.
"use strict";

const PS = (() => {
  const params = new URLSearchParams(location.search);
  let lang = params.get("lang");
  if (!lang) {
    try { lang = localStorage.getItem("ps_lang"); } catch (_) { /* storage blocked */ }
  }
  if (!lang) lang = (navigator.language || "de").toLowerCase().startsWith("de") ? "de" : "en";
  if (lang !== "en") lang = "de";

  function setLang(l) {
    try { localStorage.setItem("ps_lang", l); } catch (_) { /* ignore */ }
    const u = new URL(location.href);
    u.searchParams.set("lang", l);
    location.href = u.toString();
  }

  const dict = { de: {}, en: {} };
  function add(de, en) { Object.assign(dict.de, de); Object.assign(dict.en, en); }
  function t(key, l) {
    const d = dict[l || lang];
    return d[key] !== undefined ? d[key] : key;
  }

  // Fills elements that carry data-i18n="key" (text) or data-i18n-ph="key" (placeholder).
  // The German text in the HTML is the default; only keys with a translation are replaced.
  function apply(root) {
    const d = dict[lang];
    (root || document).querySelectorAll("[data-i18n]").forEach((el) => {
      if (d[el.dataset.i18n] !== undefined) el.textContent = d[el.dataset.i18n];
    });
    (root || document).querySelectorAll("[data-i18n-ph]").forEach((el) => {
      if (d[el.dataset.i18nPh] !== undefined) el.placeholder = d[el.dataset.i18nPh];
    });
    document.documentElement.lang = lang;
    document.querySelectorAll("[data-lang-link]").forEach((a) => {
      a.textContent = lang === "de" ? "English" : "Deutsch";
      a.addEventListener("click", (e) => { e.preventDefault(); setLang(lang === "de" ? "en" : "de"); });
    });
  }

  // el("div", {class: "x", onclick: fn}, "text", child) — never parses HTML.
  function el(tag, attrs, ...children) {
    const n = document.createElement(tag);
    for (const [k, v] of Object.entries(attrs || {})) {
      if (v === undefined || v === null || v === false) continue;
      if (k.startsWith("on") && typeof v === "function") n.addEventListener(k.slice(2), v);
      else if (k === "class") n.className = v;
      else if (k === "value") n.value = v;
      else if (k === "checked") n.checked = !!v;
      else n.setAttribute(k, v === true ? "" : v);
    }
    for (const c of children.flat()) {
      if (c === undefined || c === null || c === false) continue;
      n.append(c instanceof Node ? c : document.createTextNode(String(c)));
    }
    return n;
  }

  async function api(method, path, body, token) {
    const headers = { "Content-Type": "application/json" };
    if (token) headers["X-Edit-Token"] = token;
    const res = await fetch(path, { method, headers, body: body ? JSON.stringify(body) : undefined });
    let data = null;
    try { data = await res.json(); } catch (_) { /* no body */ }
    if (!res.ok) {
      const err = new Error((data && data.error) || res.statusText);
      err.status = res.status;
      throw err;
    }
    return data;
  }

  function wsURL(path) {
    return (location.protocol === "https:" ? "wss://" : "ws://") + location.host + path;
  }

  // Show ID from /edit/{id} or /host/{id}; edit token from the #fragment
  // (fragments never reach the server or its logs).
  function showRef() {
    const id = location.pathname.split("/").filter(Boolean)[1] || "";
    const token = decodeURIComponent(location.hash.replace(/^#/, ""));
    return { id, token };
  }

  const SHAPES = ["▲", "◆", "●", "■"];

  return { lang, t, add, apply, el, api, wsURL, showRef, SHAPES, setLang };
})();

PS.add(
  {
    brand_made: "Erstellt mit", brand_name: "PartyShow",
    footer_imprint: "Impressum", footer_privacy: "Datenschutz",
  },
  {
    brand_made: "Made with", brand_name: "PartyShow",
    footer_imprint: "Imprint", footer_privacy: "Privacy",
  },
);

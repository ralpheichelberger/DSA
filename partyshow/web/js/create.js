"use strict";

PS.add(
  {
    c_err_company: "Bitte gebt einen Firmennamen ein.",
    c_err_content: "Bitte gebt eine Webseite oder mindestens einen Fakt ein.",
    c_err_consent: "Bitte bestätigt die Datenschutzhinweise.",
    c_err_site: "Die Webseite konnte nicht gelesen werden. Bitte ergänzt ein paar Insider-Fakten.",
  },
  {
    c_title: "Create your show",
    c_lead: "The more insider facts, the more personal the show. Takes about a minute.",
    c_company: "Company name", c_company_ph: "e.g. Sample Ltd",
    c_website: "Website (optional)",
    c_website_hint: "We only read the home page to ask questions about your company.",
    c_occasion: "Occasion",
    o_christmas: "Christmas party", o_teamevent: "Team event / summer party", o_farewell: "Farewell / retirement",
    o_birthday: "Birthday", o_wedding: "Wedding",
    c_facts: "Insider facts – one per line",
    c_facts_ph: "Tom brings cake every Monday\nThe coffee machine broke four times\nWe moved into the new office in March\nOur biggest client is a bakery",
    c_facts_hint: "Only nice things everyone may know. No health, no salary, nothing embarrassing.",
    c_teams: "Teams or departments (optional, comma separated)", c_teams_ph: "Sales, Engineering, Accounting",
    c_language: "Show language",
    c_safe: "Safe mode: no questions about age, weight, salary, alcohol, relationships, religion or politics",
    c_consent: "I have read the", c_consent2: "and only enter information I'm allowed to share.",
    c_submit: "Generate show",
    c_working: "The AI is writing your show … this takes up to a minute.",
    c_err_company: "Please enter a company name.",
    c_err_content: "Please enter a website or at least one fact.",
    c_err_consent: "Please confirm the privacy notice.",
    c_err_site: "The website could not be read. Please add a few insider facts.",
  },
);

PS.apply();
document.getElementById("language").value = PS.lang;

const $ = (id) => document.getElementById(id);

$("form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const err = $("error");
  err.textContent = "";
  const facts = $("facts").value.split("\n").map((s) => s.trim()).filter(Boolean);
  const intake = {
    company_name: $("company").value.trim(),
    website_url: $("website").value.trim(),
    facts,
    teams: $("teams").value.split(",").map((s) => s.trim()).filter(Boolean),
    occasion: $("occasion").value,
    language: $("language").value,
    safe_mode: $("safe").checked,
  };
  if (!intake.company_name) { err.textContent = PS.t("c_err_company"); return; }
  if (!intake.website_url && facts.length === 0) { err.textContent = PS.t("c_err_content"); return; }
  if (!$("consent").checked) { err.textContent = PS.t("c_err_consent"); return; }

  $("submit").disabled = true;
  $("working").classList.remove("hidden");
  try {
    const res = await PS.api("POST", "/api/shows", intake);
    let url = "/edit/" + res.id + "?lang=" + PS.lang;
    if (res.warnings && res.warnings.includes("website_unreadable")) url += "&w=site";
    location.href = url + "#" + encodeURIComponent(res.edit_token);
  } catch (ex) {
    err.textContent = ex.status === 422 ? PS.t("c_err_site") : ex.message;
    $("submit").disabled = false;
    $("working").classList.add("hidden");
  }
});

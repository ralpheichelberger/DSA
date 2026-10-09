"use strict";

PS.add(
  {},
  {
    hero_title: "Your company party as a TV game show.",
    hero_lead: "About your company, your inside jokes, your year. Ready in 10 minutes – no host to book, no prep. Everyone plays on their own phone.",
    cta_create: "Create your show", cta_demo: "Watch the demo",
    hero_free: "Try it free with up to 15 players.",
    s1_t: "1. Enter", s1_d: "Your website and 10–20 insider facts: “Tom always brings cake”, “We moved offices in March”.",
    s2_t: "2. Review", s2_d: "The AI writes 5 rounds with about 20 questions. Change or delete any question.",
    s3_t: "3. Play", s3_d: "Put the show on a projector or TV, scan the QR code, go. With host voice, timer and live leaderboard.",
    cmp_t: "Why not just Kahoot or a hired host?",
    cmp_diy: "Quiz tools (write it yourself)", cmp_agency: "Hired host / agency",
    cmp_r1: "Questions about your company", cmp_auto: "automatic", cmp_self: "write them yourself (hours)", cmp_briefing: "after a briefing",
    cmp_r2: "Preparation", cmp_10: "10 minutes", cmp_hours: "several hours", cmp_weeks: "booking, scheduling",
    cmp_r3: "Price", cmp_p1: "from €149", cmp_p2: "cheap, but costs your time", cmp_p3: "about €200–1,200 or per head",
    price_t: "Pricing",
    pl_free: "Trial", pl_free_p: "€0", pl_free_d: "up to 15 players",
    pl_small: "Team", pl_small_d: "up to 50 players",
    pl_medium: "Company", pl_medium_d: "up to 200 players",
    pl_large: "Large", pl_large_d: "up to 500 players",
    price_note: "One-time payment per show, no subscription. Includes all questions, host voice and live leaderboard.",
    faq_t: "FAQ",
    faq1_q: "Do we need an app?", faq1_a: "No. Players scan the QR code and play in their phone's browser.",
    faq2_q: "Could something embarrassing come up about colleagues?", faq2_a: "Safe mode is on by default and filters sensitive topics like age, weight, salary or alcohol. You also review every question before the party.",
    faq3_q: "What do we need on site?", faq3_a: "A laptop connected to a projector or TV, internet, and speakers for the host voice.",
    faq4_q: "What happens to our data?", faq4_a: "Servers in the EU. We store only what the show needs and delete everything automatically after 60 days.",
  },
);

PS.apply();
if (PS.lang === "en") document.title = "PartyShow – your company party as a game show";

document.getElementById("cta").href = "/create?lang=" + PS.lang;
document.getElementById("demo").addEventListener("click", async () => {
  const btn = document.getElementById("demo");
  btn.disabled = true;
  try {
    const res = await PS.api("POST", "/api/demo?lang=" + PS.lang);
    location.href = "/host/" + res.id + "?demo=1#" + encodeURIComponent(res.edit_token);
  } catch (e) {
    document.getElementById("demo-error").textContent = e.message;
    btn.disabled = false;
  }
});

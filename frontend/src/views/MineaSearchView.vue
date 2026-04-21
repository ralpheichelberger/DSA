<script setup>
import { computed, reactive, ref } from 'vue'
import { useApi, primaryProductLink } from '../composables/useApi'

const api = useApi()
const form = reactive({
  country: 'US',
  sort_by: '-publication_date',
  media_types: ['video'],
  media_types_excludes: [],
  ad_publication_date: 'last_14_days',
  ad_publication_date_range_from: '',
  ad_publication_date_range_to: '',
  ad_is_active: ['active'],
  ad_is_active_excludes: [],
  ad_languages: ['de'],
  ad_languages_excludes: [],
  ad_countries: ['GB', 'DE', 'FR', 'ES', 'IT', 'PL', 'NL', 'AT', 'BE', 'CZ', 'DK', 'FI', 'GR', 'HU', 'IE', 'LU', 'PT', 'RO', 'SE', 'SK'],
  ad_countries_excludes: [],
  ad_days_running: [7, 30],
  ctas: ['SHOP_NOW', 'BUY_NOW', 'LEARN_MORE'],
  ctas_excludes: [],
  only_eu: false,
  collapse: 'advertiser__id',
  cpm_value: 9,
  exclude_bad_data: true,
  start_page: 1,
  pages: 3,
  per_page: 20,
  limit: 0,
  /** Saved rows get this niche label (agent discovery niches are separate — see AGENT_DISCOVERY_NICHES). */
  scrape_niche: '',
  /** Meta Ads Library text search (matches browser URL `query=`). */
  query: '',
  /** Which fields to match against (browser default for keyword search: `adCopy`). */
  q_search_targets: 'adCopy',
})

const options = {
  countries: ['US', 'EU'],
  sortBy: ['-publication_date'],
  mediaTypes: ['video', 'image', 'carousel'],
  publicationDate: [
    { label: 'Last 7 days', value: 'last_7_days' },
    { label: 'Last 1 week', value: 'last_1_week' },
    { label: 'Last 14 days', value: 'last_14_days' },
    { label: 'Last 30 days', value: 'last_30_days' },
    { label: 'Last 1 month', value: 'last_1_month' },
    { label: 'Custom range', value: 'custom_range' },
    { label: 'Legacy: last_2_weeks', value: 'last_2_weeks' },
  ],
  adIsActive: ['active'],
  adLanguages: ['ar', 'bg', 'zh', 'hr', 'cs', 'da', 'nl', 'en', 'et', 'fi', 'fr', 'de', 'el', 'he', 'hi', 'hu', 'id', 'it', 'ja', 'ko', 'lv', 'lt', 'ms', 'nb', 'pl', 'pt', 'ro', 'ru', 'sr', 'sk', 'sl', 'es', 'sv', 'tl', 'th', 'tr', 'uk', 'vi'],
  adCountries: ['GB', 'DE', 'FR', 'ES', 'IT', 'PL', 'NL', 'AT', 'BE', 'CZ', 'DK', 'FI', 'GR', 'HU', 'IE', 'LU', 'PT', 'RO', 'SE', 'SK'],
  adDaysRunning: [7, 30],
  ctas: ['SHOP_NOW', 'BUY_NOW', 'LEARN_MORE'],
  collapse: ['advertiser__id'],
}

const countrySearch = ref('')
const languageSearch = ref('')
const filteredCountries = computed(() => {
  const q = countrySearch.value.trim().toUpperCase()
  if (!q) return options.adCountries
  return options.adCountries.filter((c) => c.includes(q))
})
const filteredLanguages = computed(() => {
  const q = languageSearch.value.trim().toLowerCase()
  if (!q) return options.adLanguages
  return options.adLanguages.filter((l) => l.toLowerCase().includes(q))
})

function setCountryMode(country, mode) {
  form.ad_countries = form.ad_countries.filter((c) => c !== country)
  form.ad_countries_excludes = form.ad_countries_excludes.filter((c) => c !== country)
  if (mode === 'include') form.ad_countries.push(country)
  if (mode === 'exclude') form.ad_countries_excludes.push(country)
}

function includeAllVisibleCountries() {
  for (const c of filteredCountries.value) setCountryMode(c, 'include')
}

function excludeAllVisibleCountries() {
  for (const c of filteredCountries.value) setCountryMode(c, 'exclude')
}

function clearAllVisibleCountries() {
  for (const c of filteredCountries.value) setCountryMode(c, 'clear')
}

function setLanguageMode(language, mode) {
  form.ad_languages = form.ad_languages.filter((l) => l !== language)
  form.ad_languages_excludes = form.ad_languages_excludes.filter((l) => l !== language)
  if (mode === 'include') form.ad_languages.push(language)
  if (mode === 'exclude') form.ad_languages_excludes.push(language)
}

function includeAllVisibleLanguages() {
  for (const l of filteredLanguages.value) setLanguageMode(l, 'include')
}

function excludeAllVisibleLanguages() {
  for (const l of filteredLanguages.value) setLanguageMode(l, 'exclude')
}

function clearAllVisibleLanguages() {
  for (const l of filteredLanguages.value) setLanguageMode(l, 'clear')
}

async function runSearch() {
  await api.runMineaSearch({
    country: form.country,
    sort_by: form.sort_by,
    media_types: form.media_types,
    media_types_excludes: form.media_types_excludes,
    ad_publication_date: form.ad_publication_date,
    ad_publication_date_range:
      form.ad_publication_date === 'custom_range' && form.ad_publication_date_range_from && form.ad_publication_date_range_to
        ? [form.ad_publication_date_range_from, form.ad_publication_date_range_to]
        : [],
    ad_is_active: form.ad_is_active,
    ad_is_active_excludes: form.ad_is_active_excludes,
    ad_languages: form.ad_languages,
    ad_languages_excludes: form.ad_languages_excludes,
    ad_countries: form.ad_countries,
    ad_countries_excludes: form.ad_countries_excludes,
    ad_days_running: form.ad_days_running,
    ctas: form.ctas,
    ctas_excludes: form.ctas_excludes,
    only_eu: form.only_eu,
    collapse: form.collapse,
    cpm_value: Number(form.cpm_value) || 0,
    exclude_bad_data: !!form.exclude_bad_data,
    start_page: Number(form.start_page) || 1,
    pages: Number(form.pages) || 1,
    per_page: Number(form.per_page) || 20,
    limit: Number(form.limit) || 0,
    ...(form.scrape_niche ? { scrape_niche: form.scrape_niche } : {}),
    ...(form.query.trim()
      ? {
          query: form.query.trim(),
          q_search_targets: (form.q_search_targets || 'adCopy').trim() || 'adCopy',
        }
      : {}),
  })
}
</script>

<template>
  <div class="page">
    <div class="head">
      <h2 class="title">Minea Search</h2>
      <button type="button" class="btn" :disabled="api.mineaSearch.loading" @click="runSearch">
        {{ api.mineaSearch.loading ? 'Searching…' : 'Run Search' }}
      </button>
    </div>

    <div class="panel">
      <label>Keyword search (Meta library)
        <input v-model.trim="form.query" type="text" placeholder="e.g. pets — matches browser query=" />
      </label>
      <label>Search targets
        <input
          v-model.trim="form.q_search_targets"
          type="text"
          placeholder="adCopy"
          :disabled="!form.query.trim()"
        />
      </label>
      <p class="hint keyword-hint">When set, the backend sends <code>query</code> and <code>q_search_targets</code> on the search RPC (same idea as the Meta Ads Library URL). Targets default to <code>adCopy</code> when empty.</p>
      <label>Country
        <select v-model="form.country">
          <option v-for="v in options.countries" :key="v" :value="v">{{ v }}</option>
        </select>
      </label>
      <label>Sort by
        <select v-model="form.sort_by">
          <option v-for="v in options.sortBy" :key="v" :value="v">{{ v }}</option>
        </select>
      </label>
      <label>Publication date
        <select v-model="form.ad_publication_date">
          <option v-for="v in options.publicationDate" :key="v.value" :value="v.value">{{ v.label }}</option>
        </select>
      </label>
      <label v-if="form.ad_publication_date === 'custom_range'">Date from
        <input v-model="form.ad_publication_date_range_from" type="datetime-local" />
      </label>
      <label v-if="form.ad_publication_date === 'custom_range'">Date to
        <input v-model="form.ad_publication_date_range_to" type="datetime-local" />
      </label>
      <label>Media types
        <select v-model="form.media_types" multiple size="3">
          <option v-for="v in options.mediaTypes" :key="v" :value="v">{{ v }}</option>
        </select>
      </label>
      <label>Media types excludes
        <select v-model="form.media_types_excludes" multiple size="3">
          <option v-for="v in options.mediaTypes" :key="`mx-${v}`" :value="v">{{ v }}</option>
        </select>
      </label>
      <label>Ad status
        <select v-model="form.ad_is_active" multiple size="2">
          <option v-for="v in options.adIsActive" :key="v" :value="v">{{ v }}</option>
        </select>
      </label>
      <label>Ad status excludes
        <select v-model="form.ad_is_active_excludes" multiple size="2">
          <option v-for="v in options.adIsActive" :key="`ax-${v}`" :value="v">{{ v }}</option>
        </select>
      </label>
      <div class="countries-box">
        <div class="countries-head">
          <strong>Languages</strong>
          <span class="hint">{{ form.ad_languages.length }} include · {{ form.ad_languages_excludes.length }} exclude</span>
        </div>
        <input v-model="languageSearch" placeholder="Search language code (e.g. de)" />
        <div class="countries-actions">
          <button type="button" class="mini" @click="includeAllVisibleLanguages">Include visible</button>
          <button type="button" class="mini" @click="excludeAllVisibleLanguages">Exclude visible</button>
          <button type="button" class="mini" @click="clearAllVisibleLanguages">Clear visible</button>
        </div>
        <div class="countries-list">
          <div v-for="l in filteredLanguages" :key="l" class="country-row">
            <span>{{ l }}</span>
            <div class="country-buttons">
              <button type="button" :class="['mini', form.ad_languages.includes(l) ? 'mini--on' : '']" @click="setLanguageMode(l, form.ad_languages.includes(l) ? 'clear' : 'include')">Include</button>
              <button type="button" :class="['mini', form.ad_languages_excludes.includes(l) ? 'mini--on danger' : '']" @click="setLanguageMode(l, form.ad_languages_excludes.includes(l) ? 'clear' : 'exclude')">Exclude</button>
            </div>
          </div>
        </div>
      </div>
      <div class="countries-box">
        <div class="countries-head">
          <strong>Countries</strong>
          <span class="hint">{{ form.ad_countries.length }} include · {{ form.ad_countries_excludes.length }} exclude</span>
        </div>
        <input v-model="countrySearch" placeholder="Search country code (e.g. DE)" />
        <div class="countries-actions">
          <button type="button" class="mini" @click="includeAllVisibleCountries">Include visible</button>
          <button type="button" class="mini" @click="excludeAllVisibleCountries">Exclude visible</button>
          <button type="button" class="mini" @click="clearAllVisibleCountries">Clear visible</button>
        </div>
        <div class="countries-list">
          <div v-for="c in filteredCountries" :key="c" class="country-row">
            <span>{{ c }}</span>
            <div class="country-buttons">
              <button type="button" :class="['mini', form.ad_countries.includes(c) ? 'mini--on' : '']" @click="setCountryMode(c, form.ad_countries.includes(c) ? 'clear' : 'include')">Include</button>
              <button type="button" :class="['mini', form.ad_countries_excludes.includes(c) ? 'mini--on danger' : '']" @click="setCountryMode(c, form.ad_countries_excludes.includes(c) ? 'clear' : 'exclude')">Exclude</button>
            </div>
          </div>
        </div>
      </div>
      <label>Running days
        <select v-model="form.ad_days_running" multiple size="2">
          <option v-for="v in options.adDaysRunning" :key="v" :value="v">{{ v }}</option>
        </select>
      </label>
      <label>CTAs
        <select v-model="form.ctas" multiple size="3">
          <option v-for="v in options.ctas" :key="v" :value="v">{{ v }}</option>
        </select>
      </label>
      <label>CTAs excludes
        <select v-model="form.ctas_excludes" multiple size="3">
          <option v-for="v in options.ctas" :key="`tx-${v}`" :value="v">{{ v }}</option>
        </select>
      </label>
      <label>Collapse
        <select v-model="form.collapse">
          <option value="">(none)</option>
          <option v-for="v in options.collapse" :key="v" :value="v">{{ v }}</option>
        </select>
      </label>
      <label>CPM value <input v-model.number="form.cpm_value" type="number" step="0.1" /></label>
      <label>Start page <input v-model.number="form.start_page" type="number" min="1" /></label>
      <label>Pages <input v-model.number="form.pages" type="number" min="1" /></label>
      <label>Per page <input v-model.number="form.per_page" type="number" min="1" /></label>
      <label>Limit (0 = no cap) <input v-model.number="form.limit" type="number" min="0" /></label>
      <label>Label saved rows as niche
        <input
          v-model.trim="form.scrape_niche"
          type="text"
          list="minea-scrape-niche-suggestions"
          placeholder="optional, e.g. pets"
        />
        <datalist id="minea-scrape-niche-suggestions">
          <option value="tech" />
          <option value="pets" />
        </datalist>
      </label>
      <p class="hint scrape-hint">
        <strong>Minea Search</strong> runs the live ad search (countries, languages, media type, CTAs, …) and returns ads; you can persist them to <strong>Scraped</strong>.
        This field only <em>tags</em> saved rows with a niche string in your DB so Scraped filters stay organized — it does not change what Minea returns.
        The scheduled agent uses env <span class="env">AGENT_DISCOVERY_NICHES</span> for trending/radar by niche (separate from this tab).
      </p>
      <label class="check"><input v-model="form.only_eu" type="checkbox" /> EU only</label>
      <label class="check"><input v-model="form.exclude_bad_data" type="checkbox" /> Exclude bad data</label>
    </div>

    <p v-if="api.mineaSearch.error" class="err">{{ api.mineaSearch.error }}</p>
    <p v-if="!api.mineaSearch.loading">Results: {{ api.mineaSearch.data.length }}</p>

    <div class="results">
      <article v-for="p in api.mineaSearch.data" :key="p.id" class="card">
        <a
          v-if="p.imageURL && primaryProductLink(p)"
          :href="primaryProductLink(p)"
          class="card-media"
          target="_blank"
          rel="noopener noreferrer"
        >
          <img :src="p.imageURL" alt="" />
        </a>
        <img v-else-if="p.imageURL" :src="p.imageURL" alt="" class="card-media-img" />
        <div class="meta">
          <h3>
            <a
              v-if="primaryProductLink(p)"
              :href="primaryProductLink(p)"
              class="ext"
              target="_blank"
              rel="noopener noreferrer"
            >{{ p.name || p.id }}</a>
            <template v-else>{{ p.name || p.id }}</template>
          </h3>
          <p class="sub-links" v-if="p.adURL || p.shopURL || p.landingURL">
            <a v-if="p.adURL" :href="p.adURL" target="_blank" rel="noopener noreferrer">Ad</a>
            <span v-if="p.adURL && (p.shopURL || p.landingURL)"> · </span>
            <a v-if="p.shopURL" :href="p.shopURL" target="_blank" rel="noopener noreferrer">Shop</a>
            <span v-if="p.shopURL && p.landingURL"> · </span>
            <a v-if="p.landingURL" :href="p.landingURL" target="_blank" rel="noopener noreferrer">Product</a>
          </p>
          <p>{{ p.shopifyStore || '—' }} · ads: {{ p.activeAdCount }}</p>
          <p>engagement: {{ p.engagementScore }}</p>
          <p class="id">{{ p.id }}</p>
        </div>
      </article>
    </div>
  </div>
</template>

<style scoped>
.page { display: grid; gap: 1rem; }
.head { display:flex; justify-content:space-between; align-items:center; }
.title { margin:0; }
.btn { padding:0.45rem 1rem; border-radius:var(--radius); border:1px solid var(--color-border); background:var(--color-accent); color:#fff; }
.panel { display:grid; grid-template-columns: repeat(3, minmax(0,1fr)); gap:0.75rem; background:var(--color-surface); border:1px solid var(--color-border); border-radius:var(--radius); padding:0.75rem; }
.scrape-hint { grid-column: 1 / -1; margin: 0; font-size: 0.8rem; color: var(--color-text-muted); line-height: 1.45; }
.scrape-hint .env { font-family: ui-monospace, monospace; font-size: 0.75rem; }
.keyword-hint { grid-column: 1 / -1; margin: -0.25rem 0 0 0; font-size: 0.75rem; color: var(--color-text-muted); line-height: 1.4; }
.keyword-hint code { font-size: 0.7rem; }
label { display:grid; gap:0.3rem; font-size:0.85rem; }
input, select { padding:0.45rem 0.55rem; border:1px solid var(--color-border); border-radius:8px; background:var(--color-bg); color:var(--color-text); }
select[multiple] { min-height: 5.5rem; }
.check { display:flex; align-items:center; gap:0.5rem; }
.countries-box { border: 1px solid var(--color-border); border-radius: 8px; padding: 0.6rem; display: grid; gap: 0.5rem; background: var(--color-bg); }
.countries-head { display: flex; justify-content: space-between; align-items: center; }
.hint { color: var(--color-text-muted); font-size: 0.75rem; }
.countries-actions { display:flex; gap:0.4rem; flex-wrap: wrap; }
.countries-list { max-height: 220px; overflow: auto; border: 1px solid var(--color-border); border-radius: 6px; }
.country-row { display:flex; justify-content: space-between; align-items:center; padding: 0.35rem 0.5rem; border-bottom: 1px solid var(--color-border); font-size: 0.85rem; }
.country-row:last-child { border-bottom: none; }
.country-buttons { display:flex; gap:0.4rem; }
.mini { padding: 0.2rem 0.5rem; border:1px solid var(--color-border); background: var(--color-surface); color: var(--color-text); border-radius: 6px; font-size: 0.75rem; cursor: pointer; }
.mini--on { background: rgba(46, 125, 50, 0.2); border-color: #2e7d32; }
.danger.mini--on { background: rgba(198, 40, 40, 0.2); border-color: #c62828; }
.err { color:#c62828; margin:0; }
.results { display:grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap:0.75rem; }
.card { border:1px solid var(--color-border); border-radius:var(--radius); background:var(--color-surface); overflow:hidden; }
.card-media, .card-media-img { display:block; line-height:0; }
.card img { width:100%; height:140px; object-fit:cover; display:block; background:var(--color-bg); }
.ext { color: var(--color-accent); text-decoration: none; }
.ext:hover { text-decoration: underline; }
.sub-links { margin: 0 0 0.35rem 0 !important; font-size: 0.8rem; }
.sub-links a { color: var(--color-accent); text-decoration: none; }
.sub-links a:hover { text-decoration: underline; }
.meta { padding:0.6rem; font-size:0.85rem; }
.meta h3 { margin:0 0 0.35rem 0; font-size:0.95rem; }
.meta p { margin:0.2rem 0; }
.id { color:var(--color-text-muted); font-size:0.75rem; word-break:break-all; }
@media (max-width: 900px) { .panel { grid-template-columns: 1fr; } }
</style>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useApi, primaryProductLink } from '../composables/useApi'

const api = useApi()
/** '' = all niches; otherwise passed to GET /api/minea/scraped?niche= */
const nicheFilter = ref('')

let nicheDebounceTimer
watch(nicheFilter, () => {
  clearTimeout(nicheDebounceTimer)
  nicheDebounceTimer = setTimeout(() => {
    void refresh()
  }, 350)
})

function ageDays(iso) {
  if (!iso) return '—'
  const t = new Date(iso).getTime()
  if (Number.isNaN(t)) return '—'
  return `${Math.floor((Date.now() - t) / (24 * 60 * 60 * 1000))}d`
}

async function refresh() {
  await api.getMineaScraped(nicheFilter.value)
}

onMounted(refresh)
onUnmounted(() => {
  clearTimeout(nicheDebounceTimer)
})
</script>

<template>
  <div class="page">
    <div class="head">
      <div>
        <h2 class="title">Scraped Data (Minea)</h2>
        <p class="sub">
          Complete Minea-backed rows in <strong>product_tests</strong> (<code>source_platform = minea</code>) — every
          search persist and agent discovery save lands here. Use <strong>Products</strong> for a curated winners view
          of the pipeline. Niche filter only affects this list.
        </p>
      </div>
      <div class="toolbar">
        <label class="filter">
          <span class="filter-label">Niche filter</span>
          <input
            v-model.trim="nicheFilter"
            type="text"
            list="scraped-niche-suggestions"
            placeholder="empty = all"
            @keydown.enter.prevent="refresh"
          />
          <span class="filter-hint">refetches after you type or pick a suggestion</span>
          <datalist id="scraped-niche-suggestions">
            <option value="tech" />
            <option value="pets" />
          </datalist>
        </label>
        <button type="button" class="btn" :disabled="api.scraped.loading" @click="refresh">
          {{ api.scraped.loading ? 'Loading…' : 'Refresh' }}
        </button>
      </div>
    </div>
    <p v-if="api.scraped.error" class="err">{{ api.scraped.error }}</p>
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Image</th>
            <th>Name</th>
            <th>Links</th>
            <th>Niche</th>
            <th>ID</th>
            <th>Store</th>
            <th>Supplier</th>
            <th
              title="0–100: ScoreProduct for agent-saved rows; Minea engagement is clamped into 0–100 when saved from search."
            >
              Score
            </th>
            <th>Age</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in api.scraped.data" :key="p.id">
            <td>
              <a
                v-if="p.productImageURL && primaryProductLink(p)"
                :href="primaryProductLink(p)"
                class="thumb-a"
                target="_blank"
                rel="noopener noreferrer"
              >
                <img :src="p.productImageURL" alt="" class="thumb" />
              </a>
              <img v-else-if="p.productImageURL" :src="p.productImageURL" alt="" class="thumb" />
            </td>
            <td>
              <a
                v-if="primaryProductLink(p)"
                :href="primaryProductLink(p)"
                class="ext"
                target="_blank"
                rel="noopener noreferrer"
              >{{ p.productName || '—' }}</a>
              <template v-else>{{ p.productName || '—' }}</template>
            </td>
            <td class="links">
              <template v-if="p.adURL">
                <a :href="p.adURL" target="_blank" rel="noopener noreferrer">Ad</a>
              </template>
              <template v-if="p.shopURL">
                <span v-if="p.adURL"> · </span>
                <a :href="p.shopURL" target="_blank" rel="noopener noreferrer">Shop</a>
              </template>
              <template v-if="p.landingURL">
                <span v-if="p.adURL || p.shopURL"> · </span>
                <a :href="p.landingURL" target="_blank" rel="noopener noreferrer">Product</a>
              </template>
              <span v-if="!p.adURL && !p.shopURL && !p.landingURL" class="muted">—</span>
            </td>
            <td>{{ p.niche || '—' }}</td>
            <td class="mono">{{ p.id }}</td>
            <td>{{ p.shopifyStore || '—' }}</td>
            <td class="mono">{{ p.supplier || '—' }}</td>
            <td>{{ p.score }}</td>
            <td>{{ ageDays(p.createdAt) }}</td>
          </tr>
        </tbody>
      </table>
      <p v-if="!api.scraped.loading && !api.scraped.data.length" class="empty">No scraped Minea records yet.</p>
    </div>
  </div>
</template>

<style scoped>
.page { display: grid; gap: 1rem; }
.head { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; flex-wrap: wrap; }
.title { margin: 0; }
.sub { margin: 0.35rem 0 0 0; max-width: 36rem; font-size: 0.82rem; color: var(--color-text-muted); line-height: 1.45; }
.toolbar { display: flex; align-items: center; gap: 0.75rem; }
.filter { display: flex; flex-direction: column; align-items: stretch; gap: 0.25rem; font-size: 0.85rem; color: var(--color-text-muted); }
.filter-label { font-weight: 600; color: var(--color-text); }
.filter input { min-width: 9rem; padding: 0.35rem 0.5rem; border-radius: 8px; border: 1px solid var(--color-border); background: var(--color-bg); color: var(--color-text); }
.filter-hint { font-size: 0.72rem; color: var(--color-text-muted); max-width: 12rem; line-height: 1.25; }
.btn { padding: 0.45rem 1rem; border-radius: var(--radius); border: 1px solid var(--color-border); background: var(--color-accent); color: #fff; }
.err { color: #c62828; margin: 0; }
.table-wrap { overflow-x: auto; border: 1px solid var(--color-border); border-radius: var(--radius); background: var(--color-surface); }
th, td { padding: 0.6rem 0.7rem; border-bottom: 1px solid var(--color-border); text-align: left; }
th { color: var(--color-text-muted); background: var(--color-bg); }
.thumb-a { display: inline-block; line-height: 0; border-radius: 8px; }
.thumb { width: 40px; height: 40px; object-fit: cover; border-radius: 8px; border: 1px solid var(--color-border); }
.ext { color: var(--color-accent); text-decoration: none; }
.ext:hover { text-decoration: underline; }
.links { font-size: 0.8rem; white-space: nowrap; }
.links a { color: var(--color-accent); text-decoration: none; }
.links a:hover { text-decoration: underline; }
.muted { color: var(--color-text-muted); }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 0.8rem; }
.empty { padding: 1rem; color: var(--color-text-muted); margin: 0; }
</style>

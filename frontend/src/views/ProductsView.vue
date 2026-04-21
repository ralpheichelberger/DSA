<script setup>
import { ref, onMounted, watch } from 'vue'
import StatusBadge from '../components/StatusBadge.vue'
import { useApi, primaryProductLink } from '../composables/useApi'

const api = useApi()
/** 'winners' = server-side pipeline shortlist; 'all' = full product_tests table */
const listView = ref('winners')
const expanded = ref(new Set())

function toggle(id) {
  const next = new Set(expanded.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  expanded.value = next
}

function ageDays(iso) {
  if (!iso) return '—'
  const t = new Date(iso).getTime()
  if (Number.isNaN(t)) return '—'
  const d = Math.floor((Date.now() - t) / (24 * 60 * 60 * 1000))
  return `${d}d`
}

async function refresh() {
  await api.getProducts(listView.value === 'winners' ? { view: 'winners' } : { view: 'all' })
}

watch(listView, () => {
  void refresh()
})

onMounted(refresh)
</script>

<template>
  <div class="page">
    <div class="head">
      <div>
        <h2 class="title">Products</h2>
        <p class="sub">
          By default, <strong>Winners</strong> lists promoted tests (<code>testing</code>/<code>scaling</code>) plus
          <code>watching</code> rows that already have supplier <strong>COGS</strong> and solid margin/BEROAS. Raw Minea
          scrapes (no COGS yet) are not winners here — see <strong>Scraped</strong>. Use <strong>All products</strong>
          for the full table.
        </p>
      </div>
      <div class="toolbar">
        <label class="view-toggle">
          <span class="view-label">Show</span>
          <select v-model="listView" class="view-select">
            <option value="winners">Winners (recommended)</option>
            <option value="all">All products</option>
          </select>
        </label>
        <button type="button" class="btn" :disabled="api.products.loading" @click="refresh">
          {{ api.products.loading ? 'Loading…' : 'Refresh' }}
        </button>
      </div>
    </div>
    <p v-if="api.products.error" class="err">{{ api.products.error }}</p>
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th />
            <th>Image</th>
            <th>Name</th>
            <th>Store</th>
            <th
              title="0–100: agent uses ScoreProduct (margin + signals + shipping). Minea search uses engagement from the API, scaled into the same 0–100 band when saved."
            >
              Score
            </th>
            <th>Margin %</th>
            <th>BEROAS</th>
            <th>Status</th>
            <th>Age</th>
          </tr>
        </thead>
        <tbody>
          <template v-for="p in api.products.data" :key="p.id">
            <tr class="row-main" @click="toggle(p.id)">
              <td class="chev">{{ expanded.has(p.id) ? '▼' : '▶' }}</td>
              <td class="thumb-cell">
                <a
                  v-if="p.productImageURL && primaryProductLink(p)"
                  :href="primaryProductLink(p)"
                  class="thumb-a"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  <img :src="p.productImageURL" alt="" class="thumb" loading="lazy" />
                </a>
                <img
                  v-else-if="p.productImageURL"
                  :src="p.productImageURL"
                  alt=""
                  class="thumb"
                  loading="lazy"
                />
                <div v-else class="thumb thumb--empty" />
              </td>
              <td>
                <a
                  v-if="primaryProductLink(p)"
                  :href="primaryProductLink(p)"
                  class="ext"
                  target="_blank"
                  rel="noopener noreferrer"
                >{{ p.productName }}</a>
                <template v-else>{{ p.productName }}</template>
              </td>
              <td>
                <span class="store-pill" :class="`store--${p.shopifyStore}`">{{
                  p.shopifyStore || '—'
                }}</span>
              </td>
              <td>{{ p.score }}</td>
              <td>{{ p.grossMarginPct.toFixed(1) }}%</td>
              <td>{{ p.beroas.toFixed(2) }}</td>
              <td><StatusBadge :status="p.status" /></td>
              <td>{{ ageDays(p.createdAt) }}</td>
            </tr>
            <tr v-if="expanded.has(p.id)" class="row-detail">
              <td colspan="9">
                <div class="detail">
                  <img v-if="p.productImageURL" :src="p.productImageURL" alt="" class="detail-image" loading="lazy" />
                  <p><strong>COGS</strong> €{{ p.cogsEur.toFixed(2) }}</p>
                  <p><strong>Sell price</strong> €{{ p.sellPriceEur.toFixed(2) }}</p>
                  <p><strong>Supplier</strong> {{ p.supplier || '—' }}</p>
                  <p><strong>Source</strong> {{ p.sourcePlatform || '—' }} · <strong>Niche</strong> {{ p.niche || '—' }}</p>
                  <p v-if="p.adURL || p.shopURL || p.landingURL" class="detail-links">
                    <strong>Outbound</strong>
                    <a v-if="p.adURL" :href="p.adURL" target="_blank" rel="noopener noreferrer">Ad library</a>
                    <span v-if="p.adURL && (p.shopURL || p.landingURL)"> · </span>
                    <a v-if="p.shopURL" :href="p.shopURL" target="_blank" rel="noopener noreferrer">Shop</a>
                    <span v-if="p.shopURL && p.landingURL"> · </span>
                    <a v-if="p.landingURL" :href="p.landingURL" target="_blank" rel="noopener noreferrer">Product page</a>
                  </p>
                  <p class="strategy">
                    <strong>Platform strategy</strong>
                    Use TikTok for discovery and Meta for conversion (see niche:
                    {{ p.niche }}).
                  </p>
                  <p v-if="p.killReason"><strong>Kill reason</strong> {{ p.killReason }}</p>
                </div>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
      <p v-if="!api.products.loading && !api.products.data.length" class="empty">No products yet.</p>
    </div>
  </div>
</template>

<style scoped>
.page {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
.head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}
.toolbar {
  display: flex;
  align-items: flex-end;
  gap: 0.75rem;
  flex-shrink: 0;
}
.view-toggle {
  display: grid;
  gap: 0.25rem;
  font-size: 0.82rem;
}
.view-label {
  color: var(--color-text-muted);
}
.view-select {
  min-width: 11rem;
  padding: 0.45rem 0.55rem;
  border-radius: var(--radius);
  border: 1px solid var(--color-border);
  background: var(--color-bg);
  color: var(--color-text);
}
.title {
  margin: 0;
  font-size: 1.5rem;
}
.sub {
  margin: 0.35rem 0 0 0;
  max-width: 42rem;
  font-size: 0.82rem;
  color: var(--color-text-muted);
  line-height: 1.45;
}
.btn {
  padding: 0.45rem 1rem;
  border-radius: var(--radius);
  border: 1px solid var(--color-border);
  background: var(--color-accent);
  color: #fff;
  font-weight: 600;
}
.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
.err {
  color: #c62828;
  margin: 0;
}
.table-wrap {
  overflow-x: auto;
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  background: var(--color-surface);
}
table {
  font-size: 0.9rem;
}
th,
td {
  text-align: left;
  padding: 0.65rem 0.75rem;
  border-bottom: 1px solid var(--color-border);
}
th {
  background: var(--color-bg);
  font-weight: 600;
  color: var(--color-text-muted);
}
.row-main {
  cursor: pointer;
}
.row-main:hover {
  background: var(--color-bg);
}
.chev {
  width: 2rem;
  color: var(--color-text-muted);
  user-select: none;
}
.thumb-cell {
  width: 56px;
}
.thumb-a {
  display: inline-block;
  line-height: 0;
  border-radius: 8px;
}
.ext {
  color: var(--color-accent);
  text-decoration: none;
  font-weight: 600;
}
.ext:hover {
  text-decoration: underline;
}
.thumb {
  width: 40px;
  height: 40px;
  border-radius: 8px;
  object-fit: cover;
  border: 1px solid var(--color-border);
  background: var(--color-bg);
}
.thumb--empty {
  background: var(--color-bg);
}
.store-pill {
  display: inline-block;
  padding: 0.15rem 0.45rem;
  border-radius: 6px;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: capitalize;
}
.store--tech {
  background: #e3e8ff;
  color: #3949ab;
}
.store--pets {
  background: #ffe8f0;
  color: #ad1457;
}
.row-detail td {
  background: var(--color-bg);
  border-bottom: 1px solid var(--color-border);
}
.detail {
  padding: 0.5rem 0 0.5rem 2rem;
  font-size: 0.85rem;
}
.detail-image {
  width: 64px;
  height: 64px;
  object-fit: cover;
  border-radius: 10px;
  border: 1px solid var(--color-border);
  margin-bottom: 0.5rem;
}
.detail p {
  margin: 0.35rem 0;
}
.detail-links a {
  color: var(--color-accent);
  text-decoration: none;
}
.detail-links a:hover {
  text-decoration: underline;
}
.strategy {
  color: var(--color-text-muted);
}
.empty {
  padding: 1.5rem;
  color: var(--color-text-muted);
  margin: 0;
}

@media (prefers-color-scheme: dark) {
  .store--tech {
    background: #2a3158;
    color: #b4c0ff;
  }
  .store--pets {
    background: #4a2a3d;
    color: #ffaac8;
  }
}
</style>

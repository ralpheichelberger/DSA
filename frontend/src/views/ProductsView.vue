<script setup>
import { ref, onMounted } from 'vue'
import StatusBadge from '../components/StatusBadge.vue'
import { useApi } from '../composables/useApi'

const api = useApi()
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
  await api.getProducts()
}

onMounted(refresh)
</script>

<template>
  <div class="page">
    <div class="head">
      <h2 class="title">Products</h2>
      <button type="button" class="btn" :disabled="api.products.loading" @click="refresh">
        {{ api.products.loading ? 'Loading…' : 'Refresh' }}
      </button>
    </div>
    <p v-if="api.products.error" class="err">{{ api.products.error }}</p>
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th />
            <th>Name</th>
            <th>Store</th>
            <th>Score</th>
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
              <td>{{ p.productName }}</td>
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
              <td colspan="8">
                <div class="detail">
                  <p><strong>COGS</strong> €{{ p.cogsEur.toFixed(2) }}</p>
                  <p><strong>Sell price</strong> €{{ p.sellPriceEur.toFixed(2) }}</p>
                  <p><strong>Supplier</strong> {{ p.supplier || '—' }}</p>
                  <p><strong>Source</strong> {{ p.sourcePlatform || '—' }} · <strong>Niche</strong> {{ p.niche || '—' }}</p>
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
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}
.title {
  margin: 0;
  font-size: 1.5rem;
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
.detail p {
  margin: 0.35rem 0;
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

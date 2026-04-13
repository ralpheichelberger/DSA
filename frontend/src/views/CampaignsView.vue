<script setup>
import { computed, onMounted, ref } from 'vue'
import ROASIndicator from '../components/ROASIndicator.vue'
import { useApi } from '../composables/useApi'

const api = useApi()
const productsById = ref(new Map())

const rows = computed(() => {
  const map = productsById.value
  return api.campaigns.data.map((c) => {
    const prod = map.get(c.productTestId)
    return {
      ...c,
      productName: prod?.productName ?? c.productTestId,
      beroas: prod?.beroas ?? 0,
    }
  })
})

function fmtDate(iso) {
  if (!iso) return '—'
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? '—' : d.toLocaleString()
}

async function refresh() {
  await api.getProducts()
  const m = new Map()
  for (const p of api.products.data) {
    m.set(p.id, p)
  }
  productsById.value = m
  await api.getCampaigns()
}

onMounted(refresh)
</script>

<template>
  <div class="page">
    <div class="head">
      <h2 class="title">Campaigns</h2>
      <button type="button" class="btn" :disabled="api.campaigns.loading" @click="refresh">
        {{ api.campaigns.loading ? 'Loading…' : 'Refresh' }}
      </button>
    </div>
    <p v-if="api.campaigns.error" class="err">{{ api.campaigns.error }}</p>
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Product</th>
            <th>Platform</th>
            <th>Spend €</th>
            <th>Revenue €</th>
            <th>ROAS</th>
            <th>CTR %</th>
            <th>CPA €</th>
            <th>Purchases</th>
            <th>Date</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in rows" :key="c.id">
            <td>{{ c.productName }}</td>
            <td>{{ c.platform }}</td>
            <td>{{ c.spendEur.toFixed(2) }}</td>
            <td>{{ c.revenueEur.toFixed(2) }}</td>
            <td>
              <ROASIndicator :roas="c.roas" :beroas="c.beroas" />
            </td>
            <td>{{ c.ctrPct.toFixed(2) }}</td>
            <td>{{ c.cpaEur.toFixed(2) }}</td>
            <td>{{ c.purchases }}</td>
            <td class="date">{{ fmtDate(c.snapshotDate || c.createdAt) }}</td>
          </tr>
        </tbody>
      </table>
      <p v-if="!api.campaigns.loading && !rows.length" class="empty">No active campaign snapshots.</p>
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
  font-size: 0.85rem;
}
th,
td {
  text-align: left;
  padding: 0.6rem 0.65rem;
  border-bottom: 1px solid var(--color-border);
  vertical-align: top;
}
th {
  background: var(--color-bg);
  font-weight: 600;
  color: var(--color-text-muted);
}
.date {
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}
.empty {
  padding: 1.5rem;
  color: var(--color-text-muted);
  margin: 0;
}
</style>

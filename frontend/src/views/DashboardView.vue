<script setup>
import { computed, inject, onMounted, ref, watch, nextTick } from 'vue'
import MetricCard from '../components/MetricCard.vue'
import { useApi } from '../composables/useApi'

const agentWs = inject('agentWs')
const api = useApi()

const feedRef = ref(null)

const activeProducts = computed(() => {
  const active = new Set(['watching', 'testing', 'scaling', 'paused'])
  return api.products.data.filter((p) => active.has((p.status || '').toLowerCase())).length
})

const spendThisWeek = computed(() => {
  const now = Date.now()
  const weekMs = 7 * 24 * 60 * 60 * 1000
  return api.campaigns.data
    .filter((c) => {
      const t = c.snapshotDate ? new Date(c.snapshotDate).getTime() : 0
      return now - t <= weekMs
    })
    .reduce((s, c) => s + (c.spendEur || 0), 0)
})

const bestRoas = computed(() => {
  const withSpend = api.campaigns.data.filter((c) => (c.spendEur || 0) > 0 && (c.roas || 0) > 0)
  if (!withSpend.length) return '—'
  const max = Math.max(...withSpend.map((c) => c.roas))
  return `${max.toFixed(2)}×`
})

const lessonsCount = computed(() => api.lessons.data.length)

const feedMessages = computed(() => agentWs.messages.value.slice(-10))

function typeLabel(t) {
  return t || 'message'
}

onMounted(async () => {
  await Promise.all([api.getProducts(), api.getCampaigns(), api.getLessons()])
})

watch(
  feedMessages,
  async () => {
    await nextTick()
    const el = feedRef.value
    if (el) el.scrollTop = el.scrollHeight
  },
  { deep: true },
)
</script>

<template>
  <div class="dashboard">
    <h2 class="title">Dashboard</h2>
    <section class="grid">
      <MetricCard label="Active products" :value="activeProducts" />
      <MetricCard
        label="Total spend this week (€)"
        :value="spendThisWeek.toFixed(2)"
      />
      <MetricCard label="Best ROAS" :value="bestRoas" />
      <MetricCard label="Lessons learned" :value="lessonsCount" />
    </section>

    <section class="feed-section">
      <h3 class="subtitle">Live feed</h3>
      <p v-if="api.products.loading || api.campaigns.loading" class="hint">Loading metrics…</p>
      <div ref="feedRef" class="feed">
        <article v-for="(msg, i) in feedMessages" :key="i" class="feed-card">
          <span class="feed-type">{{ typeLabel(msg.type) }}</span>
          <h4 class="feed-subject">{{ msg.Subject ?? msg.subject ?? '—' }}</h4>
          <p class="feed-body">{{ msg.Body ?? msg.body ?? '' }}</p>
        </article>
        <p v-if="!feedMessages.length" class="empty">No WebSocket messages yet. Start the agent backend.</p>
      </div>
    </section>
  </div>
</template>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}
.title {
  margin: 0;
  font-size: 1.5rem;
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 1rem;
}
.subtitle {
  margin: 0 0 0.75rem;
  font-size: 1.1rem;
}
.feed-section {
  margin-top: 0.5rem;
}
.feed {
  max-height: 420px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  padding: 0.25rem;
}
.feed-card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  padding: 0.75rem 1rem;
  box-shadow: var(--shadow);
}
.feed-type {
  display: inline-block;
  font-size: 0.7rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--color-accent);
  margin-bottom: 0.35rem;
}
.feed-subject {
  margin: 0 0 0.35rem;
  font-size: 0.95rem;
}
.feed-body {
  margin: 0;
  font-size: 0.85rem;
  color: var(--color-text-muted);
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.empty,
.hint {
  color: var(--color-text-muted);
  font-size: 0.9rem;
}
</style>

import { ref, reactive } from 'vue'

/** Dev: same-origin via Vite proxy. Prod build: hit backend on :8080 */
export const API_BASE = import.meta.env.DEV ? '' : 'http://localhost:8080'

function pick(obj, camel, pascal) {
  if (obj == null) return undefined
  if (obj[camel] != null) return obj[camel]
  if (obj[pascal] != null) return obj[pascal]
  return undefined
}

export function normalizeProduct(p) {
  return {
    id: pick(p, 'id', 'ID') ?? '',
    productName: pick(p, 'productName', 'ProductName') ?? '',
    niche: pick(p, 'niche', 'Niche') ?? '',
    shopifyStore: pick(p, 'shopifyStore', 'ShopifyStore') ?? '',
    sourcePlatform: pick(p, 'sourcePlatform', 'SourcePlatform') ?? '',
    supplier: pick(p, 'supplier', 'Supplier') ?? '',
    cogsEur: Number(pick(p, 'cogsEur', 'COGSEur') ?? 0),
    sellPriceEur: Number(pick(p, 'sellPriceEur', 'SellPriceEur') ?? 0),
    grossMarginPct: Number(pick(p, 'grossMarginPct', 'GrossMarginPct') ?? 0),
    beroas: Number(pick(p, 'beroas', 'BEROAS') ?? 0),
    shippingCostEur: Number(pick(p, 'shippingCostEur', 'ShippingCostEur') ?? 0),
    shippingDays: Number(pick(p, 'shippingDays', 'ShippingDays') ?? 0),
    status: pick(p, 'status', 'Status') ?? '',
    killReason: pick(p, 'killReason', 'KillReason') ?? '',
    score: Number(pick(p, 'score', 'Score') ?? 0),
    createdAt: pick(p, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pick(p, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

export function normalizeCampaign(c) {
  return {
    id: pick(c, 'id', 'ID') ?? '',
    productTestId: pick(c, 'productTestId', 'ProductTestID') ?? '',
    platform: pick(c, 'platform', 'Platform') ?? '',
    campaignId: pick(c, 'campaignId', 'CampaignID') ?? '',
    spendEur: Number(pick(c, 'spendEur', 'SpendEur') ?? 0),
    revenueEur: Number(pick(c, 'revenueEur', 'RevenueEur') ?? 0),
    roas: Number(pick(c, 'roas', 'ROAS') ?? 0),
    ctrPct: Number(pick(c, 'ctrPct', 'CTRPct') ?? 0),
    cpaEur: Number(pick(c, 'cpaEur', 'CPAEur') ?? 0),
    impressions: Number(pick(c, 'impressions', 'Impressions') ?? 0),
    clicks: Number(pick(c, 'clicks', 'Clicks') ?? 0),
    purchases: Number(pick(c, 'purchases', 'Purchases') ?? 0),
    daysRunning: Number(pick(c, 'daysRunning', 'DaysRunning') ?? 0),
    snapshotDate: pick(c, 'snapshotDate', 'SnapshotDate') ?? '',
    createdAt: pick(c, 'createdAt', 'CreatedAt') ?? '',
  }
}

export function normalizeLesson(l) {
  return {
    id: pick(l, 'id', 'ID') ?? '',
    category: pick(l, 'category', 'Category') ?? '',
    lesson: pick(l, 'lesson', 'Lesson') ?? '',
    confidence: Number(pick(l, 'confidence', 'Confidence') ?? 0),
    evidenceCount: Number(pick(l, 'evidenceCount', 'EvidenceCount') ?? 0),
    createdAt: pick(l, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pick(l, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

export function useApi() {
  const products = reactive({ data: [], error: null, loading: false })
  const campaigns = reactive({ data: [], error: null, loading: false })
  const lessons = reactive({ data: [], error: null, loading: false })
  const approval = reactive({ data: null, error: null, loading: false })
  const chat = reactive({ data: null, error: null, loading: false })

  async function getProducts(status) {
    products.loading = true
    products.error = null
    try {
      const q = status ? `?status=${encodeURIComponent(status)}` : ''
      const res = await fetch(`${API_BASE}/api/products${q}`)
      if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
      const raw = await res.json()
      products.data = Array.isArray(raw) ? raw.map(normalizeProduct) : []
    } catch (e) {
      products.error = e.message || String(e)
      products.data = []
    } finally {
      products.loading = false
    }
    return products
  }

  async function getCampaigns() {
    campaigns.loading = true
    campaigns.error = null
    try {
      const res = await fetch(`${API_BASE}/api/campaigns`)
      if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
      const raw = await res.json()
      campaigns.data = Array.isArray(raw) ? raw.map(normalizeCampaign) : []
    } catch (e) {
      campaigns.error = e.message || String(e)
      campaigns.data = []
    } finally {
      campaigns.loading = false
    }
    return campaigns
  }

  async function getLessons() {
    lessons.loading = true
    lessons.error = null
    try {
      const res = await fetch(`${API_BASE}/api/lessons`)
      if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
      const raw = await res.json()
      lessons.data = Array.isArray(raw) ? raw.map(normalizeLesson) : []
    } catch (e) {
      lessons.error = e.message || String(e)
      lessons.data = []
    } finally {
      lessons.loading = false
    }
    return lessons
  }

  async function sendApproval(productTestId, approved, note = '') {
    approval.loading = true
    approval.error = null
    approval.data = null
    try {
      const res = await fetch(`${API_BASE}/api/approve`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          product_test_id: productTestId,
          approved,
          note,
        }),
      })
      if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
      approval.data = { ok: true }
    } catch (e) {
      approval.error = e.message || String(e)
    } finally {
      approval.loading = false
    }
    return approval
  }

  async function sendChat(message) {
    chat.loading = true
    chat.error = null
    chat.data = null
    try {
      const res = await fetch(`${API_BASE}/api/chat`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message }),
      })
      if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
      const body = await res.json()
      chat.data = body.reply ?? body.Reply ?? ''
    } catch (e) {
      chat.error = e.message || String(e)
    } finally {
      chat.loading = false
    }
    return chat
  }

  return {
    products,
    campaigns,
    lessons,
    approval,
    chat,
    getProducts,
    getCampaigns,
    getLessons,
    sendApproval,
    sendChat,
  }
}

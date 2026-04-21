import { ref, reactive } from 'vue'

/** Dev: same-origin via Vite proxy. Prod build: hit backend on :8080 */
export const API_BASE = import.meta.env.DEV ? '' : 'http://localhost:8080'

function pick(obj, camel, pascal) {
  if (obj == null) return undefined
  if (obj[camel] != null) return obj[camel]
  if (obj[pascal] != null) return obj[pascal]
  return undefined
}

/** Best outbound URL for a row: landing page, then shop, then platform ad link. */
export function primaryProductLink(p) {
  if (!p) return ''
  const land = String(p.landingURL || '').trim()
  const shop = String(p.shopURL || '').trim()
  const ad = String(p.adURL || '').trim()
  return land || shop || ad
}

export function normalizeProduct(p) {
  return {
    id: pick(p, 'id', 'ID') ?? '',
    productName: pick(p, 'productName', 'ProductName') ?? '',
    productImageURL: pick(p, 'productImageURL', 'ProductImageURL') ?? '',
    adURL: pick(p, 'adURL', 'AdURL') ?? '',
    shopURL: pick(p, 'shopURL', 'ShopURL') ?? '',
    landingURL: pick(p, 'landingURL', 'LandingURL') ?? '',
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

export function normalizeMineaCandidate(p) {
  return {
    id: pick(p, 'id', 'ID') ?? '',
    name: pick(p, 'name', 'Name') ?? '',
    niche: pick(p, 'niche', 'Niche') ?? '',
    shopifyStore: pick(p, 'shopifyStore', 'ShopifyStore') ?? '',
    imageURL: pick(p, 'imageURL', 'ImageURL') ?? '',
    adURL: pick(p, 'adURL', 'AdURL') ?? '',
    shopURL: pick(p, 'shopURL', 'ShopURL') ?? '',
    landingURL: pick(p, 'landingURL', 'LandingURL') ?? '',
    supplierID: pick(p, 'supplierID', 'SupplierID') ?? '',
    activeAdCount: Number(pick(p, 'activeAdCount', 'ActiveAdCount') ?? 0),
    engagementScore: Number(pick(p, 'engagementScore', 'EngagementScore') ?? 0),
    estimatedSellEur: Number(pick(p, 'estimatedSellEur', 'EstimatedSellEur') ?? 0),
    platforms: pick(p, 'platforms', 'Platforms') ?? [],
  }
}

export function useApi() {
  const products = reactive({ data: [], error: null, loading: false })
  const scraped = reactive({ data: [], error: null, loading: false })
  const campaigns = reactive({ data: [], error: null, loading: false })
  const lessons = reactive({ data: [], error: null, loading: false })
  const approval = reactive({ data: null, error: null, loading: false })
  const chat = reactive({ data: null, error: null, loading: false })
  const mineaSearch = reactive({ data: [], error: null, loading: false })

  /**
   * @param {string | { status?: string, view?: 'all' | 'winners' }} [filters]
   *   Legacy: pass a string status. Prefer `{ view: 'winners' }` for the pipeline shortlist.
   */
  async function getProducts(filters) {
    products.loading = true
    products.error = null
    try {
      const params = new URLSearchParams()
      if (typeof filters === 'string' && filters) {
        params.set('status', filters)
      } else if (filters && typeof filters === 'object') {
        if (filters.status) params.set('status', String(filters.status))
        if (filters.view) params.set('view', String(filters.view))
      }
      const q = params.toString() ? `?${params.toString()}` : ''
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

  async function getMineaScraped(niche) {
    scraped.loading = true
    scraped.error = null
    try {
      const q = niche ? `?niche=${encodeURIComponent(niche)}` : ''
      const res = await fetch(`${API_BASE}/api/minea/scraped${q}`)
      if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
      const raw = await res.json()
      scraped.data = Array.isArray(raw) ? raw.map(normalizeProduct) : []
    } catch (e) {
      scraped.error = e.message || String(e)
      scraped.data = []
    } finally {
      scraped.loading = false
    }
    return scraped
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

  async function runMineaSearch(filters = {}) {
    mineaSearch.loading = true
    mineaSearch.error = null
    try {
      const res = await fetch(`${API_BASE}/api/minea/search`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(filters),
      })
      if (!res.ok) throw new Error(await res.text())
      const raw = await res.json()
      mineaSearch.data = Array.isArray(raw) ? raw.map(normalizeMineaCandidate) : []
    } catch (e) {
      mineaSearch.error = e.message || String(e)
      mineaSearch.data = []
    } finally {
      mineaSearch.loading = false
    }
    return mineaSearch
  }

  return {
    products,
    scraped,
    campaigns,
    lessons,
    approval,
    chat,
    mineaSearch,
    getProducts,
    getMineaScraped,
    getCampaigns,
    getLessons,
    sendApproval,
    sendChat,
    runMineaSearch,
  }
}

import { createRouter, createWebHistory } from 'vue-router'
import DashboardView from '../views/DashboardView.vue'
import ProductsView from '../views/ProductsView.vue'
import ScrapedView from '../views/ScrapedView.vue'
import CampaignsView from '../views/CampaignsView.vue'
import ChatView from '../views/ChatView.vue'
import MineaSearchView from '../views/MineaSearchView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', name: 'dashboard', component: DashboardView },
    { path: '/products', name: 'products', component: ProductsView },
    { path: '/scraped', name: 'scraped', component: ScrapedView },
    { path: '/campaigns', name: 'campaigns', component: CampaignsView },
    { path: '/minea-search', name: 'minea-search', component: MineaSearchView },
    { path: '/chat', name: 'chat', component: ChatView },
  ],
})

export default router

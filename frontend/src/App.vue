<script setup>
import { provide } from 'vue'
import { RouterLink, RouterView } from 'vue-router'
import { useWebSocket } from './composables/useWebSocket'

const agentWs = useWebSocket()
provide('agentWs', agentWs)
</script>

<template>
  <div class="app">
    <header class="header">
      <h1 class="brand">Dropship Agent</h1>
      <nav class="nav">
        <RouterLink to="/" active-class="link--active">Dashboard</RouterLink>
        <RouterLink to="/products" active-class="link--active">Products</RouterLink>
        <RouterLink to="/campaigns" active-class="link--active">Campaigns</RouterLink>
        <RouterLink to="/chat" active-class="link--active">Chat</RouterLink>
      </nav>
      <div class="status" :title="agentWs.isConnected ? 'WebSocket connected' : 'WebSocket disconnected'">
        <span class="dot" :class="agentWs.isConnected ? 'dot--on' : 'dot--off'" />
        <span class="status-text">{{ agentWs.isConnected ? 'Live' : 'Offline' }}</span>
      </div>
    </header>
    <main class="main">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.app {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}
.header {
  display: flex;
  align-items: center;
  gap: 1.5rem;
  flex-wrap: wrap;
  padding: 0.85rem 1.5rem;
  background: var(--color-surface);
  border-bottom: 1px solid var(--color-border);
  box-shadow: var(--shadow);
}
.brand {
  margin: 0;
  font-size: 1.15rem;
  font-weight: 700;
  color: var(--color-accent);
}
.nav {
  display: flex;
  gap: 1rem;
  flex: 1;
}
.nav a {
  color: var(--color-text-muted);
  font-weight: 500;
  padding: 0.35rem 0;
  border-bottom: 2px solid transparent;
}
.nav a:hover {
  color: var(--color-accent);
}
.link--active {
  color: var(--color-accent) !important;
  border-bottom-color: var(--color-accent) !important;
}
.status {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  font-size: 0.85rem;
  color: var(--color-text-muted);
}
.dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}
.dot--on {
  background: #2e7d32;
  box-shadow: 0 0 6px #4caf50;
}
.dot--off {
  background: #c62828;
}
.main {
  flex: 1;
  padding: 1.25rem 1.5rem 2rem;
  max-width: 1200px;
  width: 100%;
  margin: 0 auto;
}
</style>

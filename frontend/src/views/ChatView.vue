<script setup>
import { ref, watch, inject, nextTick, onMounted } from 'vue'
import { marked } from 'marked'
import { useApi } from '../composables/useApi'

const agentWs = inject('agentWs')
const api = useApi()

const input = ref('')
const thread = ref([])
const listRef = ref(null)
let wsCursor = 0

onMounted(() => {
  wsCursor = agentWs.messages.value.length
})

marked.setOptions({ breaks: true })

function renderMd(text) {
  if (!text) return ''
  return marked.parse(String(text))
}

async function send() {
  const text = input.value.trim()
  if (!text || api.chat.loading) return
  input.value = ''
  thread.value.push({ role: 'user', text })
  await scrollBottom()
  await api.sendChat(text)
  if (api.chat.error) {
    thread.value.push({
      role: 'agent',
      badge: false,
      html: `<p class="err">${api.chat.error}</p>`,
    })
  } else {
    thread.value.push({
      role: 'agent',
      badge: false,
      html: renderMd(api.chat.data || ''),
    })
  }
  await scrollBottom()
}

async function scrollBottom() {
  await nextTick()
  const el = listRef.value
  if (el) el.scrollTop = el.scrollHeight
}

function onKeydown(e) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    send()
  }
}

watch(
  () => agentWs.messages.value.length,
  async () => {
    const list = agentWs.messages.value
    while (wsCursor < list.length) {
      const m = list[wsCursor++]
      const t = String(m.type ?? m.Type ?? '').toLowerCase()
      if (t === 'report' || t === 'alert') {
        const sub = m.Subject ?? m.subject ?? ''
        const body = m.Body ?? m.body ?? ''
        const md = `**${sub}**\n\n${body}`
        thread.value.push({
          role: 'agent',
          badge: true,
          html: renderMd(md),
        })
        await scrollBottom()
      }
    }
  },
)
</script>

<template>
  <div class="page">
    <h2 class="title">Chat</h2>
    <p class="hint">Ask the agent about strategy, margins, or campaigns. Reports and alerts from the agent also appear here.</p>
    <div ref="listRef" class="thread">
      <div
        v-for="(msg, i) in thread"
        :key="i"
        class="bubble-row"
        :class="msg.role === 'user' ? 'bubble-row--user' : 'bubble-row--agent'"
      >
        <div class="bubble" :class="msg.role === 'user' ? 'bubble--user' : 'bubble--agent'">
          <span v-if="msg.badge" class="agent-badge">[Agent]</span>
          <div v-if="msg.role === 'agent'" class="md" v-html="msg.html" />
          <template v-else>{{ msg.text }}</template>
        </div>
      </div>
      <p v-if="api.chat.loading" class="loading">Thinking…</p>
    </div>
    <div class="composer">
      <textarea
        v-model="input"
        class="input"
        rows="2"
        placeholder="Message… (Enter to send)"
        :disabled="api.chat.loading"
        @keydown="onKeydown"
      />
      <button type="button" class="btn" :disabled="api.chat.loading || !input.trim()" @click="send">
        Send
      </button>
    </div>
  </div>
</template>

<style scoped>
.page {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 8rem);
  max-height: 720px;
}
.title {
  margin: 0 0 0.35rem;
  font-size: 1.5rem;
}
.hint {
  margin: 0 0 1rem;
  font-size: 0.9rem;
  color: var(--color-text-muted);
}
.thread {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  padding: 0.5rem 0;
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  background: var(--color-surface);
  margin-bottom: 0.75rem;
}
.bubble-row {
  display: flex;
  width: 100%;
}
.bubble-row--user {
  justify-content: flex-end;
}
.bubble-row--agent {
  justify-content: flex-start;
}
.bubble {
  max-width: min(85%, 520px);
  padding: 0.65rem 0.85rem;
  border-radius: 12px;
  font-size: 0.95rem;
  line-height: 1.45;
}
.bubble--user {
  background: var(--color-accent);
  color: #fff;
  margin-right: 0.75rem;
}
.bubble--agent {
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  margin-left: 0.75rem;
}
.agent-badge {
  display: block;
  font-size: 0.7rem;
  font-weight: 700;
  color: var(--color-accent);
  margin-bottom: 0.35rem;
}
.loading {
  margin: 0 0.75rem;
  font-size: 0.85rem;
  color: var(--color-text-muted);
}
.composer {
  display: flex;
  gap: 0.65rem;
  align-items: flex-end;
}
.input {
  flex: 1;
  resize: vertical;
  min-height: 2.5rem;
  padding: 0.55rem 0.75rem;
  border-radius: var(--radius);
  border: 1px solid var(--color-border);
  background: var(--color-surface);
  color: var(--color-text);
  font-family: inherit;
  font-size: 0.95rem;
}
.btn {
  padding: 0.55rem 1.1rem;
  border-radius: var(--radius);
  border: none;
  background: var(--color-accent);
  color: #fff;
  font-weight: 600;
}
.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.md :deep(p) {
  margin: 0 0 0.5rem;
}
.md :deep(p:last-child) {
  margin-bottom: 0;
}
.md :deep(strong) {
  color: var(--color-text);
}
.md :deep(code) {
  font-size: 0.85em;
  padding: 0.1rem 0.25rem;
  border-radius: 4px;
  background: var(--color-bg);
}
.md :deep(.err) {
  color: #c62828;
  margin: 0;
}
</style>

import { ref, computed, onUnmounted } from 'vue'

const MAX_MESSAGES = 50

function wsURL() {
  if (import.meta.env.DEV) {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${proto}//${location.host}/ws`
  }
  return 'ws://localhost:8080/ws'
}

export function useWebSocket() {
  const messages = ref([])
  const isConnected = ref(false)
  let socket = null
  let reconnectTimer = null
  let stopped = false

  const lastMessage = computed(() =>
    messages.value.length ? messages.value[messages.value.length - 1] : null,
  )

  function pushMessage(raw) {
    let parsed
    try {
      parsed = typeof raw === 'string' ? JSON.parse(raw) : raw
    } catch {
      parsed = { type: 'raw', body: String(raw) }
    }
    messages.value = [...messages.value, parsed].slice(-MAX_MESSAGES)
  }

  function connect() {
    if (stopped) return
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    try {
      socket = new WebSocket(wsURL())
    } catch (e) {
      scheduleReconnect()
      return
    }

    socket.onopen = () => {
      isConnected.value = true
    }

    socket.onclose = () => {
      isConnected.value = false
      socket = null
      scheduleReconnect()
    }

    socket.onerror = () => {
      isConnected.value = false
    }

    socket.onmessage = (ev) => {
      pushMessage(ev.data)
    }
  }

  function scheduleReconnect() {
    if (stopped) return
    if (reconnectTimer) return
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      connect()
    }, 3000)
  }

  function disconnect() {
    stopped = true
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    if (socket) {
      socket.onclose = null
      socket.close()
      socket = null
    }
    isConnected.value = false
  }

  connect()

  onUnmounted(() => {
    disconnect()
  })

  return {
    messages,
    isConnected,
    lastMessage,
    disconnect,
    reconnect: connect,
  }
}

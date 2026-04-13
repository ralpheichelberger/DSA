<script setup>
import { computed } from 'vue'

const props = defineProps({
  roas: { type: Number, default: 0 },
  beroas: { type: Number, default: 0 },
})

const tier = computed(() => {
  const r = props.roas
  const b = props.beroas
  if (!b || b <= 0) return 'neutral'
  if (r < b) return 'bad'
  if (r < b * 1.25) return 'mid'
  return 'good'
})
</script>

<template>
  <span class="wrap">
    <span class="roas" :class="`roas--${tier}`">{{ roas.toFixed(2) }}×</span>
    <span class="ref" title="Break-even ROAS">ref {{ beroas > 0 ? beroas.toFixed(2) : '—' }}</span>
  </span>
</template>

<style scoped>
.wrap {
  display: inline-flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.1rem;
}
.roas {
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}
.roas--bad {
  color: #c62828;
}
.roas--mid {
  color: #e65100;
}
.roas--good {
  color: #2e7d32;
}
.roas--neutral {
  color: var(--color-text);
}

@media (prefers-color-scheme: dark) {
  .roas--bad {
    color: #ff8a80;
  }
  .roas--mid {
    color: #ffcc80;
  }
  .roas--good {
    color: #a5d6a7;
  }
}

.ref {
  font-size: 0.7rem;
  color: var(--color-text-muted);
  font-variant-numeric: tabular-nums;
}
</style>

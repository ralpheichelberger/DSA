<script setup>
defineProps({
  label: { type: String, required: true },
  value: { type: [String, Number], required: true },
  trend: { type: String, default: '' },
})
</script>

<template>
  <article class="card">
    <p class="label">{{ label }}</p>
    <p class="value">{{ value }}</p>
    <p v-if="trend" class="trend">{{ trend }}</p>
  </article>
</template>

<style scoped>
.card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  padding: 1rem 1.15rem;
  box-shadow: var(--shadow);
}
.label {
  margin: 0 0 0.35rem;
  font-size: 0.8rem;
  color: var(--color-text-muted);
  font-weight: 500;
}
.value {
  margin: 0;
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--color-accent);
}
.trend {
  margin: 0.35rem 0 0;
  font-size: 0.8rem;
  color: var(--color-text-muted);
}
</style>

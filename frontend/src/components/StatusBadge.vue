<script setup>
defineProps({
  status: {
    type: String,
    default: '',
  },
})

const map = {
  watching: 'badge--gray',
  testing: 'badge--blue',
  scaling: 'badge--green',
  killed: 'badge--red',
  paused: 'badge--amber',
}
</script>

<template>
  <span class="badge" :class="map[status?.toLowerCase()] || 'badge--gray'">
    {{ status || '—' }}
  </span>
</template>

<style scoped>
.badge {
  display: inline-block;
  padding: 0.2rem 0.55rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: capitalize;
}
.badge--gray {
  background: #e8e9ef;
  color: #3d4159;
}
.badge--blue {
  background: #dbe4ff;
  color: #2f4ad0;
}
.badge--green {
  background: #d3f0dc;
  color: #1e6b3a;
}
.badge--red {
  background: #ffd6d6;
  color: #b42318;
}
.badge--amber {
  background: #ffe8c4;
  color: #a15c07;
}

@media (prefers-color-scheme: dark) {
  .badge--gray {
    background: #2e3348;
    color: #c4c8dc;
  }
  .badge--blue {
    background: #2a3558;
    color: #9eb4ff;
  }
  .badge--green {
    background: #1e3d2e;
    color: #8fd9a8;
  }
  .badge--red {
    background: #4a2528;
    color: #ff9a90;
  }
  .badge--amber {
    background: #3d3420;
    color: #ffc46b;
  }
}
</style>

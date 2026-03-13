<script lang="ts">
  import type { StepListItem } from '../lib/api.js'

  let {
    steps,
    activeStepId,
    onNavigate,
  }: {
    steps: StepListItem[]
    activeStepId: string
    onNavigate: (id: string) => void
  } = $props()
</script>

<nav class="py-2">
  {#each steps as step}
    <button
      class="w-full text-left px-4 py-3 flex items-center gap-3 transition-colors
        {step.id === activeStepId ? 'bg-gray-800 text-white' : 'text-gray-400 hover:text-gray-200'}
        {!step.accessible ? 'opacity-40 cursor-not-allowed' : 'hover:bg-gray-800/50 cursor-pointer'}"
      disabled={!step.accessible}
      onclick={() => step.accessible && onNavigate(step.id)}
    >
      <!-- Completion indicator -->
      <span class="flex-shrink-0 w-5 h-5 rounded-full border flex items-center justify-center text-xs
        {step.completed
          ? 'bg-green-500 border-green-500 text-white'
          : step.accessible
            ? 'border-gray-600'
            : 'border-gray-700'}">
        {#if step.completed}✓{/if}
      </span>

      <!-- Title -->
      <span class="flex-1 text-sm leading-tight">{step.title}</span>

      <!-- Lock icon for inaccessible steps -->
      {#if !step.accessible}
        <span class="flex-shrink-0 text-gray-600 text-xs">🔒</span>
      {/if}
    </button>
  {/each}
</nav>

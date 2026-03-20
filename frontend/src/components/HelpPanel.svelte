<script lang="ts">
  import { renderMarkdown } from '../lib/markdown.js'
  import type { StepListItem } from '../lib/api.js'

  let {
    stepId,
    step,
  }: {
    stepId: string
    step: StepListItem | null
  } = $props()

  type Mode = 'hints' | 'explain' | 'solve'

  let activeMode = $state<Mode | null>(null)
  let content = $state('')
  let loading = $state(false)
  let error = $state<string | null>(null)

  const modeLabels: Record<Mode, string> = {
    hints: 'Hints',
    explain: 'Explain',
    solve: 'Solve',
  }

  function hasModeAvailable(mode: Mode): boolean {
    if (!step) return false
    switch (mode) {
      case 'hints': return step.hasHints
      case 'explain': return step.hasExplain
      case 'solve': return step.hasSolve
    }
  }

  const availableModes = $derived(
    (['hints', 'explain', 'solve'] as Mode[]).filter(m => hasModeAvailable(m))
  )

  async function toggleHelp(mode: Mode) {
    if (activeMode === mode) {
      activeMode = null
      return
    }
    activeMode = mode
    loading = true
    content = ''
    error = null

    try {
      const res = await fetch(`/api/steps/${stepId}/llm/help?mode=${mode}`, {
        method: 'POST',
      })
      if (!res.ok) {
        throw new Error(`${res.status} ${res.statusText}`)
      }

      const reader = res.body!.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      let accumulated = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() ?? ''

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            const data = line.slice(6).trim()
            if (data === '[DONE]') {
              break
            }
            // Decode JSON string
            try {
              const decoded = JSON.parse(data) as string
              accumulated += decoded
              content = accumulated
            } catch {
              // Non-JSON data — treat as literal
              accumulated += data
              content = accumulated
            }
          }
        }
      }
    } catch (e) {
      error = String(e)
    } finally {
      loading = false
    }
  }

  // Reset when step changes
  $effect(() => {
    stepId
    activeMode = null
    content = ''
    error = null
  })
</script>

{#if availableModes.length > 0}
  <div class="border-t border-gray-800 mt-6 pt-6">
    <h3 class="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-3">Help</h3>

    <!-- Mode buttons -->
    <div class="flex gap-2 mb-4">
      {#each availableModes as mode}
        <button
          class="px-3 py-1.5 text-xs font-medium rounded transition-colors
            {activeMode === mode
              ? 'bg-blue-600 text-white'
              : 'bg-gray-800 text-gray-400 hover:text-gray-200 hover:bg-gray-700'}"
          onclick={() => toggleHelp(mode)}
        >
          {activeMode === mode ? 'Hide' : 'Show'} {modeLabels[mode]}
        </button>
      {/each}
    </div>

    <!-- Content area -->
    {#if activeMode}
      {#if error}
        <p class="text-red-400 text-sm">{error}</p>
      {:else if content}
        <div class="prose prose-invert prose-sm max-w-none bg-gray-900/50 rounded p-4">
          {@html renderMarkdown(content)}
        </div>
      {/if}
    {/if}
  </div>
{/if}

<script lang="ts">
  import { onMount } from 'svelte'
  import { api, type Command } from '../lib/api.js'

  let commands = $state<Command[]>([])
  let error = $state<string | null>(null)

  async function refresh() {
    try {
      commands = await api.getCommands(200)
      error = null
    } catch (e) {
      error = String(e)
    }
  }

  // Poll while mounted; the parent unmounts this component when its tab is not
  // active, so the interval is torn down automatically via the returned cleanup.
  onMount(() => {
    refresh()
    const id = setInterval(refresh, 2000)
    return () => clearInterval(id)
  })
</script>

<div class="h-full overflow-y-auto bg-gray-950 p-4 font-mono text-xs">
  {#if error}
    <p class="text-red-400">{error}</p>
  {:else if commands.length === 0}
    <p class="text-gray-500">No commands yet. Run something in the terminal.</p>
  {:else}
    <ul class="space-y-1">
      {#each commands as c}
        <li class="flex items-start gap-2">
          <span class="text-gray-500 flex-shrink-0">{c.ts.slice(11, 19)}</span>
          <span class="flex-1 break-all text-gray-200">{c.cmd}</span>
          <span class="flex-shrink-0 {c.exit === 0 ? 'text-green-500' : 'text-red-400'}">
            {c.exit}
          </span>
        </li>
      {/each}
    </ul>
  {/if}
</div>

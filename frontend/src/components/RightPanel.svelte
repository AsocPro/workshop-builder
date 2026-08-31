<script lang="ts">
  import Terminal from './Terminal.svelte'
  import CommandHistory from './CommandHistory.svelte'
  import Recordings from './Recordings.svelte'

  type Tab = 'terminal' | 'commands' | 'recordings'
  let tab = $state<Tab>('terminal')

  const tabs: { id: Tab; label: string }[] = [
    { id: 'terminal', label: 'Terminal' },
    { id: 'commands', label: 'Commands' },
    { id: 'recordings', label: 'Recordings' },
  ]
</script>

<div class="flex h-full flex-col">
  <div class="flex flex-shrink-0 border-b border-gray-800 text-sm">
    {#each tabs as t}
      <button
        class="px-4 py-2 {tab === t.id
          ? 'border-b-2 border-blue-500 text-white'
          : 'text-gray-400 hover:text-gray-200'}"
        onclick={() => (tab = t.id)}
      >
        {t.label}
      </button>
    {/each}
  </div>
  <div class="relative min-h-0 flex-1">
    <!-- Terminal stays mounted so switching tabs never tears down the session. -->
    <div class="absolute inset-0" class:hidden={tab !== 'terminal'}>
      <Terminal />
    </div>
    {#if tab === 'commands'}
      <div class="absolute inset-0"><CommandHistory /></div>
    {:else if tab === 'recordings'}
      <div class="absolute inset-0"><Recordings /></div>
    {/if}
  </div>
</div>

<script lang="ts">
  import { onMount } from 'svelte'
  import { api, type RecordingInfo } from '../lib/api.js'
  import * as AsciinemaPlayer from 'asciinema-player'
  import 'asciinema-player/dist/bundle/asciinema-player.css'

  let recs = $state<RecordingInfo[]>([])
  let selected = $state<string | null>(null)
  let error = $state<string | null>(null)
  let playerEl: HTMLDivElement
  let player: { dispose: () => void } | null = null

  async function load() {
    try {
      recs = await api.listRecordings()
      error = null
    } catch (e) {
      error = String(e)
    }
  }

  $effect(() => {
    if (selected && playerEl) {
      player?.dispose()
      player = AsciinemaPlayer.create(api.recordingURL(selected), playerEl)
    }
    return () => {
      player?.dispose()
      player = null
    }
  })

  onMount(load)
</script>

<div class="flex h-full flex-col bg-gray-950">
  <div class="border-b border-gray-800 p-2">
    {#if error}
      <p class="text-red-400 text-xs">{error}</p>
    {:else if recs.length === 0}
      <p class="text-gray-500 text-xs">No recordings yet.</p>
    {:else}
      <ul class="flex flex-wrap gap-2 text-xs">
        {#each recs as r}
          <li>
            <button
              class="rounded px-2 py-1 {selected === r.filename ? 'bg-blue-700 text-white' : 'bg-gray-800 text-gray-300 hover:bg-gray-700'}"
              onclick={() => (selected = r.filename)}
            >
              {r.startTime.slice(11, 19)} · {(r.size / 1024).toFixed(1)}kb
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
  <div class="flex-1 overflow-auto p-2">
    <div bind:this={playerEl}></div>
    {#if !selected}
      <p class="text-gray-500 text-xs">Select a session to replay.</p>
    {/if}
  </div>
</div>

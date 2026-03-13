<script lang="ts">
  import { onMount } from 'svelte'
  import { api, type StepListItem } from '../lib/api.js'
  import { renderMarkdown, processMermaid } from '../lib/markdown.js'
  import ValidateButton from './ValidateButton.svelte'
  import Terminal from './Terminal.svelte'

  let {
    stepId,
    step,
    onValidated,
  }: {
    stepId: string
    step: StepListItem | null
    onValidated: () => void
  } = $props()

  let html = $state('')
  let loading = $state(false)
  let error = $state<string | null>(null)
  let contentEl: HTMLElement

  async function loadContent(id: string) {
    loading = true
    error = null
    try {
      const md = await api.getStepContent(id)
      html = renderMarkdown(md)
    } catch (e) {
      error = String(e)
    } finally {
      loading = false
    }
  }

  $effect(() => {
    if (stepId) loadContent(stepId)
  })

  $effect(() => {
    if (html && contentEl) {
      processMermaid(contentEl)
    }
  })
</script>

<div class="flex flex-1 overflow-hidden">
  <!-- Left: markdown content (scrollable) -->
  <div class="w-1/2 overflow-y-auto border-r border-gray-800 px-8 py-8">
    <!-- Step title -->
    {#if step}
      <div class="mb-6 flex items-center gap-4">
        <h2 class="text-xl font-semibold text-white">{step.title}</h2>
        {#if step.completed}
          <span class="text-xs bg-green-900 text-green-300 px-2 py-1 rounded">Completed</span>
        {/if}
      </div>
    {/if}

    <!-- Loading / error states -->
    {#if loading}
      <p class="text-gray-500">Loading…</p>
    {:else if error}
      <p class="text-red-400">{error}</p>
    {:else}
      <!-- Markdown content -->
      <div
        bind:this={contentEl}
        class="prose prose-invert prose-sm max-w-none"
      >{@html html}</div>

      <!-- Validate button (shown if step has goss and not yet completed) -->
      {#if step?.hasGoss && !step?.completed}
        <div class="mt-8 pt-8 border-t border-gray-800">
          <ValidateButton {stepId} onSuccess={onValidated} />
        </div>
      {:else if step?.completed}
        <div class="mt-8 pt-8 border-t border-gray-800">
          <p class="text-green-400 text-sm">✓ This step is complete.</p>
        </div>
      {/if}
    {/if}
  </div>

  <!-- Right: terminal (full height) -->
  <div class="w-1/2 h-full">
    <Terminal />
  </div>
</div>

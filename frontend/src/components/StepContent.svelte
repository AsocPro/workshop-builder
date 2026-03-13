<script lang="ts">
  import { onMount } from 'svelte'
  import { api, type StepListItem, type ValidateResult } from '../lib/api.js'
  import { renderMarkdown, processMermaid } from '../lib/markdown.js'
  import ValidateButton from './ValidateButton.svelte'
  import Terminal from './Terminal.svelte'

  let {
    stepId,
    step,
    onValidated,
    onContinue,
    hasNext,
  }: {
    stepId: string
    step: StepListItem | null
    onValidated: () => void
    onContinue: () => void
    hasNext: boolean
  } = $props()

  let html = $state('')
  let loading = $state(false)
  let error = $state<string | null>(null)
  let contentEl: HTMLElement
  let lastResult = $state<ValidateResult | null>(null)

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

      <!-- Validate / complete controls -->
      {#if step}
        <div class="mt-8 pt-8 border-t border-gray-800 space-y-4">
          <!-- Completion banner — independent of validate button -->
          {#if step.completed}
            <div class="flex items-center gap-4">
              {#if hasNext}
                <button
                  class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded font-medium text-sm transition-colors"
                  onclick={onContinue}
                >
                  Continue →
                </button>
                <p class="text-green-400 text-sm">✓ This step is complete.</p>
              {:else}
                <p class="text-green-400 text-sm">✓ All steps are complete.</p>
              {/if}
            </div>
          {/if}

          <!-- Last validate result (shown after button unmounts on success) -->
          {#if step.completed && lastResult}
            <div class="space-y-2">
              <div class="flex items-center gap-2 px-3 py-2 rounded text-sm font-medium bg-green-900/50 text-green-300">
                <span>✓</span>
                <span>All checks passed!</span>
              </div>
              {#if lastResult.checks && lastResult.checks.length > 0}
                <div class="space-y-1">
                  {#each lastResult.checks as check}
                    <div class="flex items-start gap-2 text-xs">
                      <span class="text-green-400 flex-shrink-0 mt-0.5">✓</span>
                      <span class="text-gray-300">{check.name}</span>
                    </div>
                  {/each}
                </div>
              {/if}
            </div>
          {/if}

          {#if step.hasGoss && !step.completed}
            <ValidateButton {stepId} onSuccess={(r) => { lastResult = r; onValidated() }} />
          {:else if !step.completed}
            <button
              class="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded font-medium text-sm transition-colors"
              onclick={async () => { await api.validate(stepId); onValidated() }}
            >
              Mark as Complete
            </button>
          {/if}
        </div>
      {/if}
    {/if}
  </div>

  <!-- Right: terminal (full height) -->
  <div class="w-1/2 h-full">
    <Terminal />
  </div>
</div>

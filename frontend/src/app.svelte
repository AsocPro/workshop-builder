<script lang="ts">
  import { onMount } from 'svelte'
  import { api, type WorkshopState, type StepListItem } from './lib/api.js'
  import StepList from './components/StepList.svelte'
  import StepContent from './components/StepContent.svelte'
  import ManagementLink from './components/ManagementLink.svelte'

  let state = $state<WorkshopState | null>(null)
  let steps = $state<StepListItem[]>([])
  let activeStepId = $state<string | null>(null)
  let error = $state<string | null>(null)

  async function loadInitialState() {
    try {
      ;[state, steps] = await Promise.all([api.getState(), api.listSteps()])
      activeStepId = state.activeStep
    } catch (e) {
      error = String(e)
    }
  }

  async function onNavigate(stepId: string) {
    try {
      await api.navigate(stepId)
      activeStepId = stepId
    } catch (e) {
      console.error('navigate error:', e)
    }
  }

  async function onValidated() {
    // Refresh state after successful validation
    state = await api.getState()
    steps = await api.listSteps()
  }

  const hasNext = $derived(
    steps.slice(steps.findIndex(s => s.id === activeStepId) + 1).some(s => s.accessible)
  )

  async function onContinue() {
    const idx = steps.findIndex(s => s.id === activeStepId)
    const next = steps.slice(idx + 1).find(s => s.accessible)
    if (next) await onNavigate(next.id)
  }

  onMount(loadInitialState)
</script>

<div class="flex h-screen bg-gray-950 text-gray-100 overflow-hidden">
  <!-- Sidebar -->
  <aside class="w-64 flex-shrink-0 border-r border-gray-800 flex flex-col overflow-hidden">
    <div class="p-4 border-b border-gray-800">
      <h1 class="text-sm font-semibold text-gray-300 uppercase tracking-wider">
        {state?.navigationMode ?? 'Workshop'}
      </h1>
    </div>
    <div class="flex-1 overflow-y-auto">
      {#if steps.length > 0}
        <StepList
          {steps}
          activeStepId={activeStepId ?? ''}
          onNavigate={onNavigate}
        />
      {:else}
        <p class="p-4 text-gray-500 text-sm">Loading steps…</p>
      {/if}
    </div>
    {#if state?.managementURL}
      <div class="p-4 border-t border-gray-800">
        <ManagementLink url={state.managementURL} />
      </div>
    {/if}
  </aside>

  <!-- Main content -->
  <main class="flex-1 flex min-w-0 overflow-hidden">
    {#if error}
      <div class="p-8 text-red-400">Error: {error}</div>
    {:else if activeStepId}
      <StepContent
        stepId={activeStepId}
        step={steps.find(s => s.id === activeStepId) ?? null}
        onValidated={onValidated}
        onContinue={onContinue}
        hasNext={hasNext}
      />
    {:else}
      <div class="p-8 text-gray-500">Select a step to begin.</div>
    {/if}
  </main>
</div>

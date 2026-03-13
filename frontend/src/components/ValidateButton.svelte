<script lang="ts">
  import { api, type ValidateResult } from '../lib/api.js'

  let {
    stepId,
    onSuccess,
  }: {
    stepId: string
    onSuccess: (result: ValidateResult) => void
  } = $props()

  let loading = $state(false)
  let result = $state<ValidateResult | null>(null)
  let errorMsg = $state<string | null>(null)

  async function validate() {
    loading = true
    result = null
    errorMsg = null

    try {
      result = await api.validate(stepId)
      if (result.passed) {
        onSuccess(result)
      }
    } catch (e) {
      errorMsg = String(e)
    } finally {
      loading = false
    }
  }
</script>

<div class="space-y-3">
  <button
    class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded font-medium text-sm
           disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
    disabled={loading}
    onclick={validate}
  >
    {loading ? 'Validating…' : 'Validate'}
  </button>

  {#if errorMsg}
    <p class="text-red-400 text-sm">{errorMsg}</p>
  {/if}

  {#if result}
    <div class="space-y-2">
      <!-- Overall result banner -->
      <div class="flex items-center gap-2 px-3 py-2 rounded text-sm font-medium
        {result.passed ? 'bg-green-900/50 text-green-300' : 'bg-red-900/50 text-red-300'}">
        <span>{result.passed ? '✓' : '✗'}</span>
        <span>{result.passed ? 'All checks passed!' : 'Some checks failed.'}</span>
      </div>

      <!-- Per-check results -->
      {#if result.checks && result.checks.length > 0}
        <div class="space-y-1 text-sm">
          {#each result.checks as check}
            <div class="flex items-start gap-2 text-xs">
              <span class="{check.passed ? 'text-green-400' : 'text-red-400'} flex-shrink-0 mt-0.5">
                {check.passed ? '✓' : '✗'}
              </span>
              <div>
                <span class="text-gray-300">{check.name}</span>
                {#if !check.passed && check.summary}
                  <p class="text-red-400 mt-0.5">{check.summary}</p>
                {/if}
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
</div>

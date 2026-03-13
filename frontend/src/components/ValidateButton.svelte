<script lang="ts">
  let {
    stepId,
    onSuccess,
  }: {
    stepId: string
    onSuccess: () => void
  } = $props()

  let loading = $state(false)
  let message = $state<string | null>(null)

  async function validate() {
    loading = true
    message = null
    try {
      const result = await fetch(`/api/steps/${stepId}/validate`, { method: 'POST' })
      if (result.status === 501) {
        message = 'Validation not yet implemented (M8).'
        return
      }
      const data = await result.json()
      message = data.passed ? '✓ All checks passed!' : '✗ Some checks failed.'
      if (data.passed) onSuccess()
    } catch (e) {
      message = `Error: ${e}`
    } finally {
      loading = false
    }
  }
</script>

<div>
  <button
    class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded font-medium text-sm
           disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
    disabled={loading}
    onclick={validate}
  >
    {loading ? 'Validating…' : 'Validate'}
  </button>
  {#if message}
    <p class="mt-2 text-sm {message.startsWith('✓') ? 'text-green-400' : 'text-yellow-400'}">
      {message}
    </p>
  {/if}
</div>

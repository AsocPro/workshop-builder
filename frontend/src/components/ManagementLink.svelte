<script lang="ts">
  let { url }: { url: string } = $props()
</script>

<a
  href={url}
  target="_blank"
  rel="noopener noreferrer"
  class="flex items-center gap-2 text-xs text-gray-500 hover:text-gray-300 transition-colors"
>
  <span>⚙</span>
  <span>Management</span>
</a>

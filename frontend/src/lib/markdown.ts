import MarkdownIt from 'markdown-it'
import hljs from 'highlight.js'
import 'highlight.js/styles/github-dark.css'
import mermaid from 'mermaid'

// Initialize mermaid
mermaid.initialize({ startOnLoad: false, theme: 'dark' })

const md = new MarkdownIt({
  html: false,
  linkify: true,
  typographer: true,
  highlight(str: string, lang: string): string {
    if (lang && hljs.getLanguage(lang)) {
      try {
        return `<pre class="hljs"><code>${hljs.highlight(str, { language: lang, ignoreIllegals: true }).value}</code></pre>`
      } catch {}
    }
    return `<pre class="hljs"><code>${md.utils.escapeHtml(str)}</code></pre>`
  },
})

// Plugins
import taskLists from 'markdown-it-task-lists'
import container from 'markdown-it-container'
import anchor from 'markdown-it-anchor'

md.use(taskLists, { enabled: true })
md.use(anchor, { permalink: anchor.permalink.ariaHidden({ placement: 'before' }) })

// Admonition containers: :::note, :::warning, :::tip
for (const type of ['note', 'warning', 'tip', 'info']) {
  md.use(container, type, {
    render(tokens: any[], idx: number) {
      if (tokens[idx].nesting === 1) {
        return `<div class="admonition admonition-${type}">\n`
      }
      return '</div>\n'
    },
  })
}

/**
 * Render markdown string to HTML.
 * After rendering, runs Mermaid on any .mermaid blocks.
 */
export function renderMarkdown(source: string): string {
  return md.render(source)
}

/**
 * After inserting rendered HTML into the DOM, call this to process Mermaid diagrams.
 */
export async function processMermaid(container: HTMLElement): Promise<void> {
  const blocks = container.querySelectorAll('code.language-mermaid')
  if (blocks.length === 0) return

  for (const block of blocks) {
    const pre = block.parentElement
    if (!pre) continue
    const graphDef = block.textContent || ''
    const div = document.createElement('div')
    div.className = 'mermaid'
    div.textContent = graphDef
    pre.replaceWith(div)
  }

  await mermaid.run({ nodes: container.querySelectorAll('.mermaid') })
}

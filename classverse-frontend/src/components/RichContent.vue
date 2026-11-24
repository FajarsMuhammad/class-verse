<script setup lang="ts">
import { h, type VNode } from 'vue'
import type { JSONContent } from '@tiptap/core'

defineProps<{ content: JSONContent }>()
const tags: Record<string, string> = { doc: 'div', paragraph: 'p', bulletList: 'ul', orderedList: 'ol', listItem: 'li', hardBreak: 'br', callout: 'aside', taskList: 'ul', taskItem: 'li' }
function render(node: JSONContent): VNode {
  if (node.type === 'text') {
    return (node.marks ?? []).reduce((child, mark) => {
      if (mark.type === 'link') {
        const href = String(mark.attrs?.href ?? '')
        return /^(https?:\/\/|mailto:)/i.test(href) ? h('a', { href, rel: 'noopener noreferrer' }, [child]) : child
      }
      const tag = { bold: 'strong', italic: 'em', code: 'code' }[mark.type]
      return tag ? h(tag, [child]) : child
    }, h('span', node.text ?? ''))
  }
  if (node.type === 'codeBlock') {
    const text = node.content?.map(n => n.text ?? '').join('') ?? ''
    return h('section', [h('button', { type: 'button', class: 'btn-secondary', onClick: async (event: Event) => {
      try { await navigator.clipboard.writeText(text); (event.target as HTMLButtonElement).textContent = 'Tersalin' }
      catch { (event.target as HTMLButtonElement).textContent = 'Gagal menyalin; pilih teks kode' }
    } }, 'Salin kode'), h('pre', [h('code', text)])])
  }
  const tag = node.type === 'heading' ? `h${[2, 3, 4].includes(Number(node.attrs?.level)) ? node.attrs?.level : 2}` : tags[node.type ?? '']
  if (!tag) return h('span')
  return h(tag, node.type === 'callout' ? { role: 'note' } : {}, [
    ...(node.type === 'taskItem' ? [h('input', { type: 'checkbox', checked: !!node.attrs?.checked, 'aria-label': 'Latihan mandiri' })] : []),
    ...(node.content?.map(render) ?? []),
  ])
}
const Tree = (props: { content: JSONContent }) => render(props.content)
</script>
<template><div class="rich-content"><Tree :content="content" /></div></template>
<style scoped>
.rich-content { overflow-wrap: anywhere; white-space: pre-wrap; line-height: var(--line-normal); }
.rich-content :deep(pre) { overflow-x: auto; padding: var(--space-3); background: var(--color-surface-alt); }
.rich-content :deep(aside) { border-left: 3px solid var(--color-primary); padding: var(--space-3); }
.rich-content :deep(h2) { font-size: var(--text-xl); }
.rich-content :deep(h3) { font-size: var(--text-lg); }
.rich-content :deep(h4) { font-size: var(--text-md); }
</style>

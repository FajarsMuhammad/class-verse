import { Node, type JSONContent } from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import TaskList from '@tiptap/extension-task-list'
import TaskItem from '@tiptap/extension-task-item'
import UniqueID from '@tiptap/extension-unique-id'

const blockTypes = ['paragraph', 'heading', 'bulletList', 'orderedList', 'listItem', 'codeBlock', 'callout', 'taskList', 'taskItem']
const Callout = Node.create({
  name: 'callout', group: 'block', content: 'paragraph+',
  parseHTML: () => [{ tag: 'aside' }], renderHTML: () => ['aside', { role: 'note' }, 0],
})

export function richExtensions() {
  return [
    StarterKit.configure({ heading: { levels: [2, 3, 4] }, horizontalRule: false, blockquote: false, strike: false, underline: false,
      link: { openOnClick: false, autolink: false, protocols: ['https', 'http', 'mailto'], HTMLAttributes: { target: null, rel: 'noopener noreferrer' } } }),
    TaskList, TaskItem.configure({ nested: false }), Callout,
    UniqueID.configure({ types: blockTypes }),
  ]
}

// Persist only schema attributes; Tiptap link presentation attributes are not stored.
export function cleanRich(node: JSONContent): JSONContent {
  return { ...node, ...(node.type==='doc'?{attrs:{schema_version:1}}:{}), ...(node.type === 'text' ? {} : { marks: undefined }),
    ...(node.marks ? { marks: node.marks.map(m => m.type === 'link' ? { type: 'link', attrs: { href: m.attrs?.href } } : { type: m.type }) } : {}),
    ...(node.content ? { content: node.content.map(cleanRich) } : {}) }
}
export function plainRich(text: string): JSONContent {
  return { type: 'doc', content: [{ type: 'paragraph', attrs: { id: crypto.randomUUID() }, ...(text ? { content: [{ type: 'text', text }] } : {}) }] }
}
export function richText(node: JSONContent): string {
  return node.text ?? (node.content?.map(richText).join(node.type === 'doc' ? '\n' : '') ?? '')
}
export function templateRich(kind: 'article' | 'steps' | 'code'): JSONContent {
  const block = (type: string, text: string, attrs = {}): JSONContent => ({ type, attrs: { id: crypto.randomUUID(), ...attrs }, content: text ? [{ type: 'text', text }] : [] })
  return { type: 'doc', content: kind === 'code'
    ? [block('heading', 'Penjelasan', { level: 2 }), block('paragraph', ''), block('codeBlock', '', { language: 'text' }), { type: 'taskList', attrs: { id: crypto.randomUUID() }, content: [{ type: 'taskItem', attrs: { id: crypto.randomUUID(), checked: false }, content: [block('paragraph', 'Latihan mandiri')] }] }]
    : kind === 'steps' ? [block('heading', 'Tujuan', { level: 2 }), block('paragraph', ''), block('heading', 'Langkah pertama', { level: 2 }), block('paragraph', '')]
      : [block('heading', 'Judul artikel', { level: 2 }), block('paragraph', '')] }
}

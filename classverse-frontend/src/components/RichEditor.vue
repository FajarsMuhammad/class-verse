<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { EditorContent, useEditor } from '@tiptap/vue-3'
import type { JSONContent } from '@tiptap/core'
import { cleanRich, plainRich, richExtensions } from './richtext'
const props = defineProps<{ modelValue?: JSONContent | null; text: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: JSONContent]; 'update:text': [value: string] }>()
const linkError = ref('')
const editor = useEditor({ extensions: richExtensions(), content: props.modelValue ?? plainRich(props.text),
  editorProps: { attributes: { role: 'textbox', 'aria-label': 'Isi materi rich text', 'aria-multiline': 'true' } },
  onUpdate: ({ editor }) => { emit('update:modelValue', cleanRich(editor.getJSON())); emit('update:text', editor.getText().slice(0, 5000)) },
})
watch(() => props.modelValue, value => { if (value && editor.value && JSON.stringify(cleanRich(editor.value.getJSON())) !== JSON.stringify(value)) editor.value.commands.setContent(value, { emitUpdate: false }) })
onBeforeUnmount(() => editor.value?.destroy())
function link() {
  const href = window.prompt('Tautan https, http, atau mailto')
  if (!href) return
  if (!/^(https?:\/\/|mailto:)/i.test(href)) { linkError.value = 'Tautan tidak aman'; return }
  linkError.value = ''
  editor.value?.chain().focus().setLink({ href }).run()
}
function removeBlock(){const e=editor.value;if(!e || e.state.selection.$from.depth<1)return;e.chain().focus().deleteNode(e.state.selection.$from.node(1).type.name).run()}
function language(event:Event){editor.value?.chain().focus().updateAttributes('codeBlock',{language:(event.target as HTMLInputElement).value}).run()}
function duplicate() {
  const e = editor.value; if (!e || e.state.selection.$from.depth<1) return
  const { $from } = e.state.selection
  const node = $from.node(1)
  if (!node) return
  const copy = (n: JSONContent): JSONContent => ({ ...n, ...(n.attrs ? { attrs: { ...n.attrs, id: crypto.randomUUID() } } : {}), ...(n.content ? { content: n.content.map(copy) } : {}) })
  e.chain().focus().insertContentAt($from.after(1), copy(node.toJSON())).run()
}
function move(delta: -1 | 1) {
  const e = editor.value; if (!e || e.state.selection.$from.depth<1) return
  const { $from } = e.state.selection
  const index = $from.index(0), doc = e.getJSON(), blocks = doc.content ?? [], target = index + delta
  if (target < 0 || target >= blocks.length) return
  e.commands.setContent({ ...doc, content: blocks.map((n, i) => i === index ? blocks[target]! : i === target ? blocks[index]! : n) })
}
</script>
<template>
  <div v-if="editor" class="rich-editor">
    <div role="toolbar" aria-label="Format materi" class="toolbar">
      <button type="button" class="btn-secondary" :aria-pressed="editor.isActive('bold')" @click="editor.chain().focus().toggleBold().run()">Tebal</button>
      <button type="button" class="btn-secondary" :aria-pressed="editor.isActive('italic')" @click="editor.chain().focus().toggleItalic().run()">Miring</button>
      <button v-for="level in ([2, 3, 4] as const)" :key="level" type="button" class="btn-secondary" @click="editor.chain().focus().toggleHeading({ level }).run()">H{{ level }}</button>
      <button type="button" class="btn-secondary" @click="editor.chain().focus().toggleBulletList().run()">Daftar</button>
      <button type="button" class="btn-secondary" @click="editor.chain().focus().toggleOrderedList().run()">Nomor</button>
      <button type="button" class="btn-secondary" @click="editor.chain().focus().toggleCodeBlock().run()">Kode</button>
      <button type="button" class="btn-secondary" @click="editor.chain().focus().toggleTaskList().run()">Latihan</button>
      <button type="button" class="btn-secondary" @click="editor.chain().focus().insertContent({ type: 'callout', content: [{ type: 'paragraph' }] }).run()">Catatan</button>
      <button type="button" class="btn-secondary" @click="link">Tautan</button>
      <button type="button" class="btn-secondary" @click="editor.chain().focus().unsetLink().run()">Lepas tautan</button>
      <button type="button" class="btn-secondary" @click="duplicate">Duplikasi blok</button>
      <button type="button" class="btn-secondary" @click="move(-1)">Naikkan blok</button>
      <button type="button" class="btn-secondary" @click="move(1)">Turunkan blok</button>
      <button type="button" class="btn-secondary" @click="removeBlock">Hapus blok</button>
      <button type="button" class="btn-secondary" @click="editor.chain().focus().undo().run()">Urungkan</button>
      <button type="button" class="btn-secondary" @click="editor.chain().focus().redo().run()">Ulangi</button>
    </div>
    <p v-if="linkError" role="alert">{{ linkError }}</p>
    <label v-if="editor.isActive('codeBlock')">Bahasa kode <input maxlength="30" :value="editor.getAttributes('codeBlock').language ?? 'text'" @input="language" /></label>
    <EditorContent :editor="editor" />
  </div>
</template>
<style scoped>
.rich-editor { min-width: 0; border: 1px solid var(--kv-line); border-radius: var(--radius-md); padding: var(--space-3); }
.toolbar { display: flex; flex-wrap: wrap; gap: var(--space-1); }
.rich-editor :deep(.tiptap) { min-height: var(--space-8); padding: var(--space-3); overflow-wrap: anywhere; }
.rich-editor :deep(.tiptap:focus-visible) { outline: 2px solid var(--color-primary); }
.rich-editor :deep(pre) { white-space: pre-wrap; background: var(--color-surface-alt); padding: var(--space-2); }
</style>

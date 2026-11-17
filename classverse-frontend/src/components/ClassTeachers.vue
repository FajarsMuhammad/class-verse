<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { classApi, type ClassTeacher, type Staff } from '@/api/classes'

// C2b: wali kelas (one) and guru mapel (many) of a class. Only admin changes them; others read.
const props = defineProps<{ orgId: string; classId: string; isAdmin: boolean }>()
const emit = defineEmits<{ changed: [] }>()
const teachers = ref<ClassTeacher[]>([])
const staff = ref<Staff[]>([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const person = ref('')
const role = ref<'homeroom' | 'subject'>('subject')
const subject = ref('')
const homeroom = computed(() => teachers.value.find(t => t.role === 'homeroom'))
const subjects = computed(() => teachers.value.filter(t => t.role === 'subject'))
const message = (e: unknown) => e instanceof Error ? e.message : 'Perubahan gagal disimpan. Coba lagi.'

async function load() {
  loading.value = true; error.value = ''
  try {
    const [t, s] = await Promise.all([classApi.teachers(props.orgId, props.classId), props.isAdmin ? classApi.staff(props.orgId) : null])
    teachers.value = t.items
    if (s) staff.value = s.items
  } catch (e) { error.value = message(e) }
  finally { loading.value = false }
}
async function save(action: () => Promise<unknown>) {
  if (saving.value) return
  saving.value = true; error.value = ''
  try { await action(); await load(); emit('changed') }
  catch (e) { error.value = message(e) }
  finally { saving.value = false }
}
async function add() {
  const who = staff.value.find(s => s.user_id === person.value)
  if (!who) return
  const current = homeroom.value
  if (role.value === 'homeroom' && current && current.user_id !== who.user_id &&
    !window.confirm(`Ganti wali kelas ${current.display_name} dengan ${who.display_name}?`)) return
  await save(async () => {
    await classApi.setTeacher(props.orgId, props.classId, who.user_id, role.value, role.value === 'subject' ? subject.value : '')
    person.value = ''; subject.value = ''; role.value = 'subject'
  })
}
async function remove(t: ClassTeacher) {
  if (!window.confirm(`Lepas ${t.display_name} dari kelas ini?`)) return
  await save(() => classApi.removeTeacher(props.orgId, props.classId, t.user_id))
}
onMounted(load)
</script>

<template>
  <div class="class-part" :aria-busy="loading || saving">
    <p v-if="error" role="alert">{{ error }}</p>
    <p v-if="loading" role="status">Memuat…</p>
    <h4 class="section-title">Wali kelas</h4>
    <ul class="list">
      <li v-if="homeroom" data-teacher="homeroom">
        <div><strong>{{ homeroom.display_name }}</strong></div>
        <button v-if="isAdmin" type="button" class="btn-secondary" :disabled="saving" :aria-label="`Lepas ${homeroom.display_name}`" @click="remove(homeroom)">Lepas</button>
      </li>
    </ul>
    <p v-if="!homeroom && !loading" class="muted">Belum ada wali kelas. Kelas tanpa wali hanya dikelola admin.</p>
    <h4 class="section-title">Guru mapel</h4>
    <ul class="list">
      <li v-for="t in subjects" :key="t.user_id" :data-teacher="t.user_id">
        <div><strong>{{ t.display_name }}</strong><p v-if="t.subject" class="meta">{{ t.subject }}</p></div>
        <button v-if="isAdmin" type="button" class="btn-secondary" :disabled="saving" :aria-label="`Lepas ${t.display_name}`" @click="remove(t)">Lepas</button>
      </li>
    </ul>
    <p v-if="!subjects.length && !loading" class="muted">Belum ada guru mapel.</p>
    <form v-if="isAdmin" class="row" data-test="add-teacher" @submit.prevent="add">
      <fieldset :disabled="saving || loading" class="row">
        <legend class="sr-only">Tambah pengajar kelas</legend>
        <label>Pengajar
          <select v-model="person" required>
            <option value="" disabled>{{ staff.length ? 'Pilih pengajar' : 'Belum ada pengajar' }}</option>
            <option v-for="s in staff" :key="s.user_id" :value="s.user_id">{{ s.display_name }}</option>
          </select>
        </label>
        <label>Peran
          <select v-model="role" data-test="teacher-role">
            <option value="subject">Guru mapel</option>
            <option value="homeroom">Wali kelas</option>
          </select>
        </label>
        <label v-if="role === 'subject'">Mata pelajaran (opsional) <input v-model="subject" maxlength="40" placeholder="Matematika" /></label>
        <button type="submit" class="btn-primary" :disabled="!person">Simpan</button>
      </fieldset>
    </form>
    <p v-else class="muted">Wali kelas dan guru mapel diatur admin sekolah.</p>
  </div>
</template>

<style scoped src="../styles/classes.css"></style>
<style scoped>
fieldset.row { display: flex; }
</style>

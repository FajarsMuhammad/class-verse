import { defineStore } from 'pinia'
import { ref } from 'vue'

// usePendingStore carries the email between /daftar, /masuk and /verifikasi
// without putting it in the URL (URLs end up in history and logs).
export const usePendingStore = defineStore('pending', () => {
  const email = ref('')
  return { email }
})

// Browser checks for the campus workspace: minimap + place card, travel into a room, seat walk, and leaving the room.
// Replaces the throwaway /tmp/kv-e2e scripts (workspace-exit, revamp-travel, seat-walk, minimap-card).
//
// Needs the dev stack running: backend (make up && make run), Mailpit, and Vite on http://localhost:5173.
//   npm run e2e                       # headless
//   HEADFUL=1 npm run e2e             # watch it
//   CHROME=/path/to/chrome npm run e2e
// It seeds its own data through the real API (one teacher, one school, one classroom per run) under @e2e.test emails,
// so it never touches existing accounts. Data is left in the dev DB; archive it from Kelola if it gets noisy.
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import puppeteer from 'puppeteer-core'

// Mailpit's UI port comes from the backend .env (MAILPIT_UI_PORT, compose default 8025). Only that key is read.
const mailpitPort = (() => {
  try {
    return /^MAILPIT_UI_PORT=(\d+)$/m.exec(readFileSync(new URL('../../classverse-backend/.env', import.meta.url), 'utf8'))?.[1]
  } catch {
    return undefined
  }
})() ?? '8025'
const BASE = process.env.BASE_URL ?? 'http://localhost:5173'
const MAILPIT = process.env.MAILPIT_URL ?? `http://localhost:${mailpitPort}`
const CHROME = process.env.CHROME ?? '/usr/bin/google-chrome'
const run = Date.now().toString(36)
const email = `guru-${run}@e2e.test`
const password = `e2e-pass-${run}`

// post sends one JSON request to the API (seeding only needs POST).
async function post(path, body, token) {
  const res = await fetch(BASE + path, {
    method: 'POST',
    headers: { 'content-type': 'application/json', ...(token ? { authorization: `Bearer ${token}` } : {}) },
    body: JSON.stringify(body),
  })
  const text = await res.text()
  if (!res.ok) throw new Error(`POST ${path}: ${res.status} ${text}`)
  return text ? JSON.parse(text) : null
}

// otp reads the newest verification code sent to `to` from Mailpit.
async function otp(to) {
  for (let i = 0; i < 40; i++) {
    const list = await (await fetch(`${MAILPIT}/api/v1/search?query=${encodeURIComponent(`to:"${to}"`)}`)).json()
    const id = list.messages?.[0]?.ID
    if (id) {
      const msg = await (await fetch(`${MAILPIT}/api/v1/message/${id}`)).json()
      const code = /(\d{6})/.exec(msg.Text ?? '')?.[1]
      if (code) return code
    }
    await new Promise((r) => setTimeout(r, 250))
  }
  throw new Error(`no OTP mail for ${to}`)
}

async function seed() {
  await post('/api/auth/register', { email, password, display_name: 'Guru E2E', birth_date: '1990-01-01' })
  const { access_token: token } = await post('/api/auth/verify-email', { email, code: await otp(email) })
  const org = await post('/api/orgs', { name: `Sekolah E2E ${run}`, slug: `e2e-${run}` }, token)
  const room = await post(`/api/orgs/${org.id}/rooms`, { name: `Kelas E2E ${run}`, map_key: 'classroom', building_key: 'kelas' }, token)
  return { org, room }
}

const step = async (name, fn) => {
  const t0 = Date.now()
  await fn()
  console.log(`ok  ${name} (${Date.now() - t0} ms)`)
}

const { org, room } = await seed()
const browser = await puppeteer.launch({
  executablePath: CHROME,
  headless: !process.env.HEADFUL,
  args: ['--enable-unsafe-swiftshader', '--use-angle=swiftshader', '--no-sandbox'],
})
const errors = []
try {
  const page = await browser.newPage()
  page.on('pageerror', (e) => errors.push(e.message))
  await page.setViewport({ width: 1440, height: 900 })
  page.setDefaultTimeout(20000)

  await step('login lands on the campus map', async () => {
    await page.goto(`${BASE}/masuk`)
    await page.locator('input[type=email]').fill(email)
    await page.locator('input[type=password]').fill(password)
    await page.locator('form button[type=submit]').click()
    // One school: the org store picks it; with more, the select page would show.
    await page.waitForSelector('canvas')
    await page.waitForSelector('svg.minimap')
    assert.ok(await page.$(`[aria-label="Menu ruang dan alat guru"]`), 'teacher rail is shown')
  })

  await step('side panel is open on entry', async () => {
    await page.waitForSelector('[data-panel="map"]', { visible: true })
    await page.reload()
    await page.waitForSelector('[data-panel="map"]', { visible: true })
  })

  await step('minimap shows the player and the place card opens from the building list', async () => {
    await page.waitForSelector('svg.minimap circle.player')
    await page.locator('details.places button ::-p-text(Gedung Kelas)').click()
    await page.waitForSelector('[data-testid="place-action"]')
    const card = await page.$eval('.place-card h2', (h) => h.textContent)
    assert.equal(card, 'Gedung Kelas')
  })

  await step('walk to the door and travel into the classroom', async () => {
    await page.locator('[data-testid="place-action"]').click() // "Menuju pintu"
    await page.waitForFunction(() => document.querySelector('[data-testid="place-action"]')?.textContent?.includes('Pilih ruang'), { timeout: 30000 })
    await page.locator('[data-testid="place-action"]').click()
    await page.locator(`li ::-p-text(${room.name})`).wait()
    await page.$$eval('.door li', (items, name) => {
      items.find((li) => li.textContent.includes(name))?.querySelector('button')?.click()
    }, room.name)
    await page.waitForSelector('.leave-link')
    assert.equal(await page.$('svg.minimap'), null, 'campus-only minimap hides inside a room')
  })

  await step('seat picker walks the avatar to a free seat', async () => {
    const seat = await page.waitForSelector('.seat-picker button:not([disabled])')
    const label = await seat.evaluate((b) => b.getAttribute('aria-label'))
    await seat.click()
    const n = label.split(':')[0]
    await page.waitForFunction((n) => document.querySelector(`.seat-picker button[aria-label^="${n}:"]`)?.getAttribute('aria-pressed') === 'true', { timeout: 30000 }, n)
  })

  await step('leave the room returns to the campus', async () => {
    await page.locator('.leave-link').click()
    await page.waitForSelector('svg.minimap', { timeout: 30000 })
    assert.equal(await page.$('.leave-link'), null)
  })

  assert.deepEqual(errors, [], 'no page errors')
  console.log(`all passed (school ${org.slug})`)
} catch (e) {
  console.error('FAILED:', e.message)
  if (errors.length) console.error('page errors:', errors)
  process.exitCode = 1
} finally {
  await browser.close()
}

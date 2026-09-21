import { QueryClient } from '@tanstack/react-query'
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api'
import { getValidatedSelf } from '@/lib/auth-query'

test('identity validation deduplicates navigation, expires, and forces re-entry checks', async () => {
  const client = new QueryClient()
  const adapter = api.defaults.adapter
  const user = { id: 7, username: 'test', role: 1, quota: 100 }
  let calls = 0
  useAuthStore.getState().auth.setUser(user)
  api.defaults.adapter = async (config) => {
    calls++
    return {
      config,
      status: 200,
      statusText: 'OK',
      headers: {},
      data: { success: true, data: user },
    }
  }
  try {
    await Promise.all([
      getValidatedSelf(client, 7),
      getValidatedSelf(client, 7),
    ])
    assert.equal(calls, 1)
    useAuthStore.getState().auth.setUser({ ...user, quota: 999 })
    await getValidatedSelf(client, 7)
    assert.equal(calls, 1)
    assert.equal(
      useAuthStore.getState().auth.user?.quota,
      999,
      'cached validation must not undo a newer wallet update'
    )
    await getValidatedSelf(client, 7, true)
    assert.equal(calls, 2, 'route re-entry must revalidate')
    client.setQueryData(
      ['auth', 'self', 7],
      { success: true, data: user },
      { updatedAt: Date.now() - 16_000 }
    )
    await getValidatedSelf(client, 7)
    assert.equal(calls, 3, 'validation expires after 15 seconds')
    const freshPage = new QueryClient()
    await getValidatedSelf(freshPage, 7)
    assert.equal(calls, 4, 'a new page must not trust persisted user metadata')
    freshPage.clear()
  } finally {
    client.clear()
    api.defaults.adapter = adapter
    useAuthStore.getState().auth.reset()
  }
})

test('a profile response arriving after logout cannot restore the user', async () => {
  const client = new QueryClient()
  const adapter = api.defaults.adapter
  const user = { id: 8, username: 'test', role: 1 }
  let release!: () => void
  let started!: () => void
  const gate = new Promise<void>((resolve) => {
    release = resolve
  })
  const ready = new Promise<void>((resolve) => {
    started = resolve
  })
  useAuthStore.getState().auth.setUser(user)
  api.defaults.adapter = async (config) => {
    started()
    await gate
    return {
      config,
      status: 200,
      statusText: 'OK',
      headers: {},
      data: { success: true, data: user },
    }
  }
  try {
    const request = getValidatedSelf(client, 8)
    await ready
    useAuthStore.getState().auth.reset()
    release()
    await request
    assert.equal(useAuthStore.getState().auth.user, null)
  } finally {
    release()
    client.clear()
    api.defaults.adapter = adapter
    useAuthStore.getState().auth.reset()
  }
})

// All fetch calls to the backend go through this module.
// Components never call fetch directly — they import functions from here.

async function request(method, path, body) {
  const opts = { method, headers: {} }
  if (body !== undefined) {
    opts.headers['Content-Type'] = 'application/json'
    opts.body = JSON.stringify(body)
  }
  const res = await fetch(path, opts)
  if (!res.ok) {
    let msg = `HTTP ${res.status}`
    try {
      const data = await res.json()
      if (data.error) msg = data.error
    } catch (_) {}
    throw new Error(msg)
  }
  if (res.status === 204 || res.status === 202) return null
  return res.json()
}

// Characters

export function getCharacters() {
  return request('GET', '/api/characters')
}

export function deleteCharacter(id) {
  return request('DELETE', `/api/characters/${id}`)
}

// Corporations

export function getCorporations() {
  return request('GET', '/api/corporations')
}

export function addCorporation({ id, name, delegate_id }) {
  return request('POST', '/api/corporations', { id, name, delegate_id })
}

export function deleteCorporation(id) {
  return request('DELETE', `/api/corporations/${id}`)
}

// Blueprints

// filters: { status?, owner_id?, owner_type?, category_id? }
export function getBlueprints(filters = {}) {
  const params = new URLSearchParams()
  if (filters.status)      params.set('status',      filters.status)
  if (filters.owner_id)    params.set('owner_id',    filters.owner_id)
  if (filters.owner_type)  params.set('owner_type',  filters.owner_type)
  if (filters.category_id) params.set('category_id', filters.category_id)
  const qs = params.toString()
  return request('GET', `/api/blueprints${qs ? `?${qs}` : ''}`)
}

export function getJobsSummary() {
  return request('GET', '/api/jobs/summary')
}

// Sync

export function postSync() {
  return request('POST', '/api/sync')
}

export function getSyncStatus() {
  return request('GET', '/api/sync/status')
}

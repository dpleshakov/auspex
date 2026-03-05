import { useState, useEffect, useCallback } from 'react'
import { getCharacters, getCorporations, patchDelegate, deleteCharacter, getBlueprints, getSyncStatus } from '../api/client.js'

const NPC_CORP_MIN = 1_000_000
const NPC_CORP_MAX = 2_000_000

function isNpcCorp(id) {
  return id >= NPC_CORP_MIN && id <= NPC_CORP_MAX
}

function formatLastSync(iso) {
  if (!iso) return '—'
  const d = new Date(iso)
  if (isNaN(d.getTime())) return '—'
  return d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

export default function CharactersPage() {
  // null = initial load not complete yet
  const [characters, setCharacters] = useState(null)
  const [corporations, setCorporations] = useState(null)
  const [blueprints, setBlueprints] = useState(null)
  const [syncStatuses, setSyncStatuses] = useState(null)
  const [error, setError] = useState(null)

  const loadData = useCallback(async () => {
    try {
      const [chars, corps, bps, syncs] = await Promise.all([
        getCharacters(),
        getCorporations(),
        getBlueprints(),
        getSyncStatus(),
      ])
      setCharacters(chars ?? [])
      setCorporations(corps ?? [])
      setBlueprints(bps ?? [])
      setSyncStatuses(syncs ?? [])
      setError(null)
    } catch (err) {
      setError(err.message)
    }
  }, [])

  useEffect(() => { loadData() }, [loadData])

  async function handleSetDelegate(corporationId, characterId) {
    try {
      await patchDelegate(corporationId, characterId)
      loadData()
    } catch (err) {
      setError(err.message)
    }
  }

  async function handleDelete(char) {
    const otherSameCorp = characters.filter(c => c.id !== char.id && c.corporation_id === char.corporation_id)
    const isLastInPlayerCorp = otherSameCorp.length === 0 && !isNpcCorp(char.corporation_id)

    let message
    if (isLastInPlayerCorp) {
      const corp = corporations.find(c => c.id === char.corporation_id)
      const corpName = corp?.name ?? char.corporation_name
      message = `Delete character ${char.name}? This is the last character in corporation ${corpName} — the corporation and all its blueprints will also be deleted.`
    } else {
      message = `Delete character ${char.name}? All their data will be removed.`
    }

    if (!window.confirm(message)) return

    try {
      await deleteCharacter(char.id)
      loadData()
    } catch (err) {
      setError(err.message)
    }
  }

  if (error && characters === null) {
    return <div className="chars-page__error">Error: {error}</div>
  }

  if (characters === null) {
    return <div className="chars-page__loading">Loading…</div>
  }

  if (error) {
    return <div className="chars-page__error">Error: {error}</div>
  }

  // Precompute blueprint counts per character
  const bpCountByChar = {}
  for (const bp of (blueprints ?? [])) {
    if (bp.owner_type === 'character') {
      bpCountByChar[bp.owner_id] = (bpCountByChar[bp.owner_id] ?? 0) + 1
    }
  }

  // Precompute last sync per character (max last_sync across all their endpoints)
  const lastSyncByChar = {}
  for (const s of (syncStatuses ?? [])) {
    if (s.owner_type === 'character') {
      const existing = lastSyncByChar[s.owner_id]
      if (!existing || new Date(s.last_sync) > new Date(existing)) {
        lastSyncByChar[s.owner_id] = s.last_sync
      }
    }
  }

  // Group characters by corporation_id, preserving insertion order.
  const corpsMap = new Map((corporations ?? []).map(c => [c.id, c]))
  const groups = new Map()
  for (const char of characters) {
    if (!groups.has(char.corporation_id)) groups.set(char.corporation_id, [])
    groups.get(char.corporation_id).push(char)
  }

  const groupEntries = [...groups.entries()]

  return (
    <div className="chars-page">
      {groupEntries.length === 0 ? (
        <div className="chars-page__empty">
          <p className="chars-page__empty-text">No characters added yet. Add a character to get started.</p>
          <a className="chars-page__add-btn" href="/auth/eve/login">+ Add character</a>
        </div>
      ) : (
        groupEntries.map(([corpId, chars], index) => {
          const npc = isNpcCorp(corpId)
          const corpName = npc
            ? chars[0].corporation_name
            : (corpsMap.get(corpId)?.name ?? chars[0].corporation_name)

          return (
            <div key={corpId} className={`chars-group${index > 0 ? ' chars-group--top-margin' : ''}${index < groupEntries.length - 1 ? ' chars-group--separated' : ''}`}>
              <table className="chars-group__table">
                <thead>
                  <tr className="chars-corp-row">
                    <th className="chars-corp-row__name" colSpan={npc ? 4 : 5}>{corpName}</th>
                  </tr>
                  <tr className="chars-thead">
                    <th className="chars-thead__th chars-thead__th--name">Name</th>
                    {!npc && <th className="chars-thead__th chars-thead__th--role">Role</th>}
                    <th className="chars-thead__th chars-thead__th--blueprints">Blueprints</th>
                    <th className="chars-thead__th chars-thead__th--last-sync">Last Sync</th>
                    <th className="chars-thead__th"></th>
                  </tr>
                </thead>
                <tbody>
                  {chars.map(char => (
                    <tr key={char.id} className="chars-row">
                      <td className="chars-row__name">{char.name}</td>
                      {!npc && (
                        <td className="chars-row__delegate">
                          {char.is_delegate ? (
                            <span className="chars-delegate-label">● Delegate</span>
                          ) : (
                            <button
                              className="chars-make-delegate-btn"
                              onClick={() => handleSetDelegate(corpId, char.id)}
                            >
                              ○ Make delegate
                            </button>
                          )}
                          {char.sync_error && (
                            <span className="chars-row__sync-error" title={char.sync_error}>
                              ⚠ no access
                            </span>
                          )}
                        </td>
                      )}
                      <td className="chars-row__blueprints">{bpCountByChar[char.id] ?? 0}</td>
                      <td className="chars-row__last-sync">{formatLastSync(lastSyncByChar[char.id])}</td>
                      <td className="chars-row__actions">
                        <button
                          className="chars-delete-btn"
                          onClick={() => handleDelete(char)}
                        >
                          Delete
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )
        })
      )}
    </div>
  )
}

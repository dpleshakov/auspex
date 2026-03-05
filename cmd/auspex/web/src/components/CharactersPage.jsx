import { useState, useEffect, useCallback } from 'react'
import { getCharacters, getCorporations, patchDelegate, deleteCharacter } from '../api/client.js'

const NPC_CORP_MIN = 1_000_000
const NPC_CORP_MAX = 2_000_000

function isNpcCorp(id) {
  return id >= NPC_CORP_MIN && id <= NPC_CORP_MAX
}

export default function CharactersPage() {
  // null = initial load not complete yet
  const [characters, setCharacters] = useState(null)
  const [corporations, setCorporations] = useState(null)
  const [error, setError] = useState(null)

  const loadData = useCallback(async () => {
    try {
      const [chars, corps] = await Promise.all([getCharacters(), getCorporations()])
      setCharacters(chars ?? [])
      setCorporations(corps ?? [])
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

  if (characters === null) {
    return <div className="chars-page__loading">Loading…</div>
  }

  if (error) {
    return <div className="chars-page__error">Error: {error}</div>
  }

  // Group characters by corporation_id, preserving insertion order.
  const corpsMap = new Map((corporations ?? []).map(c => [c.id, c]))
  const groups = new Map()
  for (const char of characters) {
    if (!groups.has(char.corporation_id)) groups.set(char.corporation_id, [])
    groups.get(char.corporation_id).push(char)
  }

  return (
    <div className="chars-page">
      {groups.size === 0 ? (
        <div className="chars-page__empty">
          No characters added. <a href="/auth/eve/login">Add a character</a> to get started.
        </div>
      ) : (
        [...groups.entries()].map(([corpId, chars]) => {
          const npc = isNpcCorp(corpId)
          const corpName = npc
            ? chars[0].corporation_name
            : (corpsMap.get(corpId)?.name ?? chars[0].corporation_name)

          return (
            <div key={corpId} className="chars-group">
              <div className="chars-group__header">{corpName}</div>
              <table className="chars-group__table">
                <tbody>
                  {chars.map(char => (
                    <tr key={char.id} className="chars-row">
                      <td className="chars-row__name">{char.name}</td>
                      {!npc && (
                        <td className="chars-row__delegate">
                          <button
                            className={`chars-delegate-btn${char.is_delegate ? ' chars-delegate-btn--active' : ''}`}
                            title={char.is_delegate ? 'Current delegate' : 'Set as delegate'}
                            disabled={char.is_delegate}
                            onClick={() => handleSetDelegate(corpId, char.id)}
                          >
                            {char.is_delegate ? '●' : '○'}
                          </button>
                          {char.sync_error && (
                            <span className="chars-row__sync-error" title={char.sync_error}>
                              ⚠ no access
                            </span>
                          )}
                        </td>
                      )}
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
      <div className="chars-page__footer">
        <a className="app-add-char-link" href="/auth/eve/login">+ Add character</a>
      </div>
    </div>
  )
}

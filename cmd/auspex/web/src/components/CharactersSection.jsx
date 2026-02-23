import { useState, useEffect } from 'react'
import { getJobsSummary } from '../api/client.js'

// EVE Online: base research slots per character (skill-dependent in game,
// hardcoded to 11 for MVP — skill data not fetched from ESI yet).
const TOTAL_SLOTS = 11

export default function CharactersSection({ summary: externalSummary }) {
  const [summary, setSummary] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  useEffect(() => {
    if (externalSummary !== undefined) {
      setSummary(externalSummary)
      setLoading(false)
      return
    }

    getJobsSummary()
      .then(data => {
        setSummary(data)
        setLoading(false)
      })
      .catch(err => {
        setError(err.message)
        setLoading(false)
      })
  }, [externalSummary])

  if (loading) {
    return (
      <div className="characters-section characters-section--loading">
        <span>Loading characters...</span>
      </div>
    )
  }

  if (error) {
    return (
      <div className="characters-section characters-section--error">
        <span>Failed to load characters: {error}</span>
      </div>
    )
  }

  const characters = summary?.characters ?? []

  if (characters.length === 0) {
    return (
      <div className="characters-section characters-section--empty">
        <span>No characters added. <a href="/auth/eve/login">Add a character</a> to get started.</span>
      </div>
    )
  }

  return (
    <div className="characters-section">
      <table className="characters-table">
        <thead>
          <tr>
            <th>Character</th>
            <th>Used Slots</th>
            <th>Total Slots</th>
            <th>Available</th>
          </tr>
        </thead>
        <tbody>
          {characters.map(char => {
            const available = TOTAL_SLOTS - char.used_slots
            const hasSlots = available > 0
            return (
              <tr
                key={char.id}
                className={hasSlots ? 'characters-table__row--available' : ''}
              >
                <td>{char.name}</td>
                <td className="characters-table__num">{char.used_slots}</td>
                <td className="characters-table__num">{TOTAL_SLOTS}</td>
                <td className={`characters-table__num ${hasSlots ? 'characters-table__available' : ''}`}>
                  {available}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}

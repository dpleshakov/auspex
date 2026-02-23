import { useState, useEffect, useRef, useCallback } from 'react'
import SummaryBar from './components/SummaryBar.jsx'
import CharactersSection from './components/CharactersSection.jsx'
import BlueprintTable from './components/BlueprintTable.jsx'
import { getBlueprints, getJobsSummary, postSync, getSyncStatus } from './api/client.js'

const AUTO_REFRESH_MS = 10 * 60 * 1000  // 10 minutes
const SYNC_POLL_MS = 2000               // 2 s between force-refresh polls
const SYNC_POLL_MAX_MS = 60_000         // give up waiting after 60 s

export default function App() {
  const [blueprints, setBlueprints] = useState(null)
  const [summary, setSummary] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [isRefreshing, setIsRefreshing] = useState(false)

  const syncPollRef = useRef(null)

  const loadData = useCallback(async () => {
    try {
      const [bps, sum] = await Promise.all([getBlueprints(), getJobsSummary()])
      setBlueprints(bps ?? [])
      setSummary(sum)
      setError(null)
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }, [])

  // Initial load + auto-poll every 10 minutes.
  useEffect(() => {
    loadData()
    const timer = setInterval(loadData, AUTO_REFRESH_MS)
    return () => clearInterval(timer)
  }, [loadData])

  // Cleanup sync poll on unmount.
  useEffect(() => {
    return () => {
      if (syncPollRef.current !== null) clearInterval(syncPollRef.current)
    }
  }, [])

  async function handleRefresh() {
    if (isRefreshing) return
    setIsRefreshing(true)

    try {
      await postSync()
    } catch {
      setIsRefreshing(false)
      return
    }

    // Record when the sync was triggered. Any last_sync timestamp after this
    // point means the forced sync cycle has completed.
    const refTime = new Date()
    const deadline = Date.now() + SYNC_POLL_MAX_MS

    syncPollRef.current = setInterval(async () => {
      // Safety valve: if no completion signal arrives within the timeout
      // (e.g. no characters added yet, or persistent ESI errors), give up
      // waiting and reload data anyway so the UI doesn't stay stuck.
      if (Date.now() >= deadline) {
        clearInterval(syncPollRef.current)
        syncPollRef.current = null
        await loadData()
        setIsRefreshing(false)
        return
      }

      try {
        const statuses = await getSyncStatus()
        const done =
          Array.isArray(statuses) &&
          statuses.some(row => row.last_sync && new Date(row.last_sync) > refTime)

        if (done) {
          clearInterval(syncPollRef.current)
          syncPollRef.current = null
          await loadData()
          setIsRefreshing(false)
        }
      } catch {
        // Ignore transient errors; keep polling.
      }
    }, SYNC_POLL_MS)
  }

  return (
    <div className="app">
      <header className="app-header">
        <span className="app-header__title">Auspex</span>
        <div className="app-header__actions">
          <a className="app-add-char-link" href="/auth/eve/login">
            + Add character
          </a>
          <button
            className={`app-refresh-btn${isRefreshing ? ' app-refresh-btn--refreshing' : ''}`}
            onClick={handleRefresh}
            disabled={isRefreshing || loading}
          >
            {isRefreshing ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>
      </header>

      {loading ? (
        <div className="app-loading">Loading…</div>
      ) : error ? (
        <div className="app-error">Error: {error}</div>
      ) : (
        <>
          <SummaryBar summary={summary} />
          <CharactersSection summary={summary} />
          <BlueprintTable blueprints={blueprints} />
        </>
      )}
    </div>
  )
}

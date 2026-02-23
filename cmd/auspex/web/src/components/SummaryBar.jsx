import { useState, useEffect } from 'react'
import { getJobsSummary } from '../api/client.js'

export default function SummaryBar({ summary: externalSummary }) {
  const [summary, setSummary] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  useEffect(() => {
    // If parent passes summary data, use it directly (for TASK-26 polling integration)
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
      <div className="summary-bar summary-bar--loading">
        <span>Loading summary...</span>
      </div>
    )
  }

  if (error) {
    return (
      <div className="summary-bar summary-bar--error">
        <span>Failed to load summary: {error}</span>
      </div>
    )
  }

  return (
    <div className="summary-bar">
      <SummaryItem
        label="Idle BPOs"
        value={summary.idle_blueprints}
        modifier={summary.idle_blueprints > 0 ? 'warn' : null}
      />
      <SummaryItem
        label="Overdue Jobs"
        value={summary.overdue_jobs}
        modifier={summary.overdue_jobs > 0 ? 'danger' : null}
      />
      <SummaryItem
        label="Completing Today"
        value={summary.completing_today}
      />
      <SummaryItem
        label="Free Research Slots"
        value={summary.free_research_slots}
      />
    </div>
  )
}

function SummaryItem({ label, value, modifier }) {
  const className = ['summary-item', modifier ? `summary-item--${modifier}` : '']
    .filter(Boolean)
    .join(' ')

  return (
    <div className={className}>
      <span className="summary-item__value">{value}</span>
      <span className="summary-item__label">{label}</span>
    </div>
  )
}

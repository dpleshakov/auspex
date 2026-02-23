import { useState, useEffect, useMemo } from 'react'
import {
  useReactTable,
  getCoreRowModel,
  flexRender,
} from '@tanstack/react-table'
import { getBlueprints } from '../api/client.js'

// Maps job.activity + job.status to a human-readable label.
function jobStatusLabel(job) {
  if (!job) return 'Idle'
  if (job.status === 'ready') return 'Ready'
  switch (job.activity) {
    case 'me_research': return 'ME Research'
    case 'te_research': return 'TE Research'
    case 'copying':     return 'Copying'
    default:            return job.activity
  }
}

// Formats an ISO date string as local date + time (short).
function formatLocalDate(isoStr) {
  if (!isoStr) return '—'
  return new Date(isoStr).toLocaleString(undefined, {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit',
  })
}

const COLUMNS = [
  {
    accessorKey: 'type_name',
    header: 'Name',
  },
  {
    accessorKey: 'category_name',
    header: 'Category',
  },
  {
    accessorKey: 'owner_name',
    header: 'Assigned',
  },
  {
    accessorKey: 'location_id',
    header: 'Location',
    cell: ({ getValue }) => getValue() || '—',
  },
  {
    accessorKey: 'me_level',
    header: 'ME%',
    cell: ({ getValue }) => `${getValue()}%`,
  },
  {
    accessorKey: 'te_level',
    header: 'TE%',
    cell: ({ getValue }) => `${getValue()}%`,
  },
  {
    id: 'status',
    header: 'Status',
    accessorFn: row => row,
    cell: ({ getValue }) => jobStatusLabel(getValue().job),
  },
  {
    id: 'end_date',
    header: 'Date End',
    accessorFn: row => row.job?.end_date ?? null,
    cell: ({ getValue }) => formatLocalDate(getValue()),
  },
]

export default function BlueprintTable({ blueprints: externalBlueprints }) {
  const [blueprints, setBlueprints] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  useEffect(() => {
    if (externalBlueprints !== undefined) {
      setBlueprints(externalBlueprints ?? [])
      setLoading(false)
      return
    }

    setLoading(true)
    getBlueprints()
      .then(data => {
        setBlueprints(data ?? [])
        setLoading(false)
      })
      .catch(err => {
        setError(err.message)
        setLoading(false)
      })
  }, [externalBlueprints])

  const columns = useMemo(() => COLUMNS, [])

  const table = useReactTable({
    data: blueprints,
    columns,
    getCoreRowModel: getCoreRowModel(),
  })

  if (loading) {
    return <div className="bp-table-state">Loading blueprints…</div>
  }

  if (error) {
    return <div className="bp-table-state bp-table-state--error">Failed to load blueprints: {error}</div>
  }

  if (blueprints.length === 0) {
    return <div className="bp-table-state bp-table-state--empty">No blueprints found.</div>
  }

  return (
    <div className="bp-table-wrapper">
      <table className="bp-table">
        <thead>
          {table.getHeaderGroups().map(hg => (
            <tr key={hg.id}>
              {hg.headers.map(header => (
                <th key={header.id} className="bp-table__th">
                  {flexRender(header.column.columnDef.header, header.getContext())}
                </th>
              ))}
            </tr>
          ))}
        </thead>
        <tbody>
          {table.getRowModel().rows.map(row => (
            <tr key={row.id} className="bp-table__row">
              {row.getVisibleCells().map(cell => (
                <td key={cell.id} className="bp-table__td">
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

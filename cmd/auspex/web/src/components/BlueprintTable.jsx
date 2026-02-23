import { useState, useEffect, useMemo } from 'react'
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  flexRender,
} from '@tanstack/react-table'
import { getBlueprints } from '../api/client.js'

// Full status label including 'Overdue' for ready + past end_date.
function getFullStatusLabel(row) {
  const job = row.job
  if (!job) return 'Idle'
  if (job.status === 'ready') {
    return new Date(job.end_date) < new Date() ? 'Overdue' : 'Ready'
  }
  switch (job.activity) {
    case 'me_research': return 'ME Research'
    case 'te_research': return 'TE Research'
    case 'copying':     return 'Copying'
    default:            return job.activity
  }
}

function isToday(isoStr) {
  if (!isoStr) return false
  const d = new Date(isoStr)
  const now = new Date()
  return d.getFullYear() === now.getFullYear() &&
    d.getMonth() === now.getMonth() &&
    d.getDate() === now.getDate()
}

// Returns extra CSS class for the table row based on job status.
function getRowClass(row) {
  const label = getFullStatusLabel(row)
  if (label === 'Overdue') return 'bp-table__row--overdue'
  if (row.job?.status === 'active' && isToday(row.job?.end_date)) return 'bp-table__row--completing-today'
  return ''
}

// Numeric priority for default sort (lower = higher in table).
function getStatusPriority(row) {
  switch (getFullStatusLabel(row)) {
    case 'Overdue':     return 0
    case 'Ready':       return 1
    case 'Idle':        return 2
    default:            return 3  // active variants
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

const STATUS_OPTIONS = ['All', 'Overdue', 'Ready', 'Idle', 'ME Research', 'TE Research', 'Copying']

const DEFAULT_SORT = [{ id: 'status', desc: false }, { id: 'end_date', desc: false }]

export default function BlueprintTable({ blueprints: externalBlueprints }) {
  const [blueprints, setBlueprints] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  const [statusFilter, setStatusFilter] = useState('All')
  const [ownerFilter, setOwnerFilter] = useState('All')
  const [categoryFilter, setCategoryFilter] = useState('All')
  const [sorting, setSorting] = useState(DEFAULT_SORT)

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

  // Unique owner and category values for dropdowns, derived from full data set.
  const owners = useMemo(() => {
    const set = new Set(blueprints.map(bp => bp.owner_name).filter(Boolean))
    return ['All', ...Array.from(set).sort()]
  }, [blueprints])

  const categories = useMemo(() => {
    const set = new Set(blueprints.map(bp => bp.category_name).filter(Boolean))
    return ['All', ...Array.from(set).sort()]
  }, [blueprints])

  // Apply filters before passing data to TanStack Table.
  const filteredBlueprints = useMemo(() => {
    return blueprints.filter(bp => {
      if (statusFilter !== 'All' && getFullStatusLabel(bp) !== statusFilter) return false
      if (ownerFilter !== 'All' && bp.owner_name !== ownerFilter) return false
      if (categoryFilter !== 'All' && bp.category_name !== categoryFilter) return false
      return true
    })
  }, [blueprints, statusFilter, ownerFilter, categoryFilter])

  const isFiltered = statusFilter !== 'All' || ownerFilter !== 'All' || categoryFilter !== 'All'

  function clearFilters() {
    setStatusFilter('All')
    setOwnerFilter('All')
    setCategoryFilter('All')
    setSorting(DEFAULT_SORT)
  }

  const columns = useMemo(() => [
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
      accessorFn: row => getFullStatusLabel(row),
      cell: ({ getValue }) => {
        const label = getValue()
        if (label === 'Idle') return <span className="bp-status--idle">{label}</span>
        return label
      },
      sortingFn: (rowA, rowB) =>
        getStatusPriority(rowA.original) - getStatusPriority(rowB.original),
    },
    {
      id: 'end_date',
      header: 'Date End',
      accessorFn: row => row.job?.end_date ?? null,
      cell: ({ getValue }) => formatLocalDate(getValue()),
      sortingFn: (rowA, rowB) => {
        const a = rowA.original.job?.end_date
        const b = rowB.original.job?.end_date
        if (!a && !b) return 0
        if (!a) return 1   // nulls last
        if (!b) return -1
        return new Date(a) - new Date(b)
      },
    },
  ], [])

  const table = useReactTable({
    data: filteredBlueprints,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    enableMultiSort: true,
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
    <div>
      <div className="bp-filters">
        <label className="bp-filters__group">
          <span className="bp-filters__label">Status</span>
          <select
            className="bp-filters__select"
            value={statusFilter}
            onChange={e => setStatusFilter(e.target.value)}
          >
            {STATUS_OPTIONS.map(s => <option key={s}>{s}</option>)}
          </select>
        </label>

        <label className="bp-filters__group">
          <span className="bp-filters__label">Owner</span>
          <select
            className="bp-filters__select"
            value={ownerFilter}
            onChange={e => setOwnerFilter(e.target.value)}
          >
            {owners.map(o => <option key={o}>{o}</option>)}
          </select>
        </label>

        <label className="bp-filters__group">
          <span className="bp-filters__label">Category</span>
          <select
            className="bp-filters__select"
            value={categoryFilter}
            onChange={e => setCategoryFilter(e.target.value)}
          >
            {categories.map(c => <option key={c}>{c}</option>)}
          </select>
        </label>

        {isFiltered && (
          <button className="bp-filters__clear" onClick={clearFilters}>
            Clear filters
          </button>
        )}
      </div>

      <div className="bp-table-wrapper">
        <table className="bp-table">
          <thead>
            {table.getHeaderGroups().map(hg => (
              <tr key={hg.id}>
                {hg.headers.map(header => {
                  const sorted = header.column.getIsSorted()
                  const canSort = header.column.getCanSort()
                  return (
                    <th
                      key={header.id}
                      className={`bp-table__th${canSort ? ' bp-table__th--sortable' : ''}`}
                      onClick={header.column.getToggleSortingHandler()}
                    >
                      {flexRender(header.column.columnDef.header, header.getContext())}
                      {sorted === 'asc' ? ' ↑' : sorted === 'desc' ? ' ↓' : ''}
                    </th>
                  )
                })}
              </tr>
            ))}
          </thead>
          <tbody>
            {table.getRowModel().rows.map(row => {
              const rowClass = getRowClass(row.original)
              return (
              <tr key={row.id} className={rowClass ? `bp-table__row ${rowClass}` : 'bp-table__row'}>
                {row.getVisibleCells().map(cell => (
                  <td key={cell.id} className="bp-table__td">
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </td>
                ))}
              </tr>
            )
          })}
          </tbody>
        </table>

        {filteredBlueprints.length === 0 && (
          <div className="bp-table-state bp-table-state--empty">
            No blueprints match the current filters.
          </div>
        )}
      </div>
    </div>
  )
}

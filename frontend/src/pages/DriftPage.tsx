import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Server, Shield, AlertTriangle, DollarSign, Percent, Eye,
  ChevronDown, ChevronRight, Filter, Ghost
} from 'lucide-react'
import { api } from '../lib/api'
import { StatCard, LoadingSpinner, EmptyState } from '../components/shared'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface DriftSummary {
  total_resources: number
  managed_by_iac: number
  shadow_resources: number
  drifted_resources: number
  compliant: number
  iac_coverage_pct: number
  drift_cost_impact: number
  shadow_cost: number
}

interface DriftedResource {
  resource_id: string
  resource_type: string
  provider: string
  region: string
  iac_state: Record<string, any> | null
  actual_state: Record<string, any> | null
  drift_type: 'modified' | 'shadow' | 'unmanaged'
  severity: string
  detected_at: string
  monthly_cost_impact: number
  description: string
}

interface DriftResponse {
  summary: DriftSummary
  drifted_resources: DriftedResource[]
}

// ---------------------------------------------------------------------------
// Badge helpers
// ---------------------------------------------------------------------------

function DriftTypeBadge({ type }: { type: string }) {
  const colors: Record<string, string> = {
    modified: 'bg-yellow-100 text-yellow-800',
    shadow: 'bg-red-100 text-red-800',
    unmanaged: 'bg-orange-100 text-orange-800',
  }
  return (
    <span className={`px-2 py-1 rounded-full text-xs font-medium capitalize ${colors[type] || 'bg-gray-100 text-gray-800'}`}>
      {type}
    </span>
  )
}

function SeverityBadge({ severity }: { severity: string }) {
  const colors: Record<string, string> = {
    low: 'bg-green-100 text-green-800',
    medium: 'bg-yellow-100 text-yellow-800',
    high: 'bg-orange-100 text-orange-800',
    critical: 'bg-red-100 text-red-800',
  }
  return (
    <span className={`px-2 py-1 rounded-full text-xs font-medium capitalize ${colors[severity] || 'bg-gray-100 text-gray-800'}`}>
      {severity}
    </span>
  )
}

// ---------------------------------------------------------------------------
// State comparison component
// ---------------------------------------------------------------------------

function StateComparison({ iacState, actualState }: {
  iacState: Record<string, any> | null
  actualState: Record<string, any> | null
}) {
  if (!iacState && !actualState) return null

  const allKeys = new Set([
    ...Object.keys(iacState || {}),
    ...Object.keys(actualState || {}),
  ])

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-3">
      {/* IaC State */}
      <div className="rounded-lg border border-blue-200 bg-blue-50/50 p-3">
        <p className="text-xs font-semibold text-blue-700 mb-2 uppercase tracking-wide">IaC State (Expected)</p>
        {iacState ? (
          <div className="space-y-1">
            {[...allKeys].map(key => {
              const iacVal = iacState[key]
              const actualVal = actualState?.[key]
              const isDiff = JSON.stringify(iacVal) !== JSON.stringify(actualVal)
              return (
                <div key={key} className="flex justify-between text-xs">
                  <span className="text-gray-600">{key}:</span>
                  <span className={`font-mono ${isDiff ? 'text-blue-700 font-semibold' : 'text-gray-700'}`}>
                    {iacVal !== undefined ? (typeof iacVal === 'object' ? JSON.stringify(iacVal) : String(iacVal)) : '-'}
                  </span>
                </div>
              )
            })}
          </div>
        ) : (
          <p className="text-xs text-gray-400 italic">Not managed by IaC</p>
        )}
      </div>

      {/* Actual State */}
      <div className="rounded-lg border border-amber-200 bg-amber-50/50 p-3">
        <p className="text-xs font-semibold text-amber-700 mb-2 uppercase tracking-wide">Actual State (Running)</p>
        {actualState ? (
          <div className="space-y-1">
            {[...allKeys].map(key => {
              const iacVal = iacState?.[key]
              const actualVal = actualState[key]
              const isDiff = JSON.stringify(iacVal) !== JSON.stringify(actualVal)
              return (
                <div key={key} className="flex justify-between text-xs">
                  <span className="text-gray-600">{key}:</span>
                  <span className={`font-mono ${isDiff ? 'text-amber-700 font-semibold' : 'text-gray-700'}`}>
                    {actualVal !== undefined ? (typeof actualVal === 'object' ? JSON.stringify(actualVal) : String(actualVal)) : '-'}
                  </span>
                </div>
              )
            })}
          </div>
        ) : (
          <p className="text-xs text-gray-400 italic">Resource not found</p>
        )}
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Main Page
// ---------------------------------------------------------------------------

export default function DriftPage() {
  const [driftTypeFilter, setDriftTypeFilter] = useState('all')
  const [severityFilter, setSeverityFilter] = useState('all')
  const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set())

  // ---- Query ----

  const { data, isLoading } = useQuery<DriftResponse>({
    queryKey: ['drift', 'summary'],
    queryFn: () => api.get('/drift/summary'),
  })

  // ---- Derived data ----

  const summary = data?.summary
  const resources = data?.drifted_resources || []

  const filteredResources = resources.filter(r => {
    if (driftTypeFilter !== 'all' && r.drift_type !== driftTypeFilter) return false
    if (severityFilter !== 'all' && r.severity !== severityFilter) return false
    return true
  })

  const toggleRow = (id: string) => {
    setExpandedRows(prev => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  // ---- Loading ----

  if (isLoading) return <LoadingSpinner />

  // ---- Render ----

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold">IaC Drift Detection</h1>
        <p className="text-gray-500 text-sm mt-1">
          Monitor drift between running infrastructure and IaC definitions (Terraform, CloudFormation)
        </p>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 lg:grid-cols-6 gap-4">
        <StatCard
          title="Total Resources"
          value={String(summary?.total_resources ?? 0)}
          icon={Server}
        />
        <StatCard
          title="IaC Managed"
          value={String(summary?.managed_by_iac ?? 0)}
          icon={Shield}
        />
        <StatCard
          title="Shadow Resources"
          value={String(summary?.shadow_resources ?? 0)}
          icon={Ghost}
        />
        <StatCard
          title="Drifted"
          value={String(summary?.drifted_resources ?? 0)}
          icon={AlertTriangle}
        />
        <StatCard
          title="IaC Coverage"
          value={`${summary?.iac_coverage_pct?.toFixed(1) ?? 0}%`}
          icon={Percent}
        />
        <StatCard
          title="Cost Impact"
          value={`$${(summary?.drift_cost_impact ?? 0).toLocaleString()}`}
          subtitle={`$${(summary?.shadow_cost ?? 0).toLocaleString()} shadow`}
          icon={DollarSign}
        />
      </div>

      {/* Filters */}
      <div className="bg-white rounded-xl shadow-sm p-4 border border-gray-100">
        <div className="flex items-center gap-4">
          <Filter size={20} className="text-gray-400" />
          <select
            value={driftTypeFilter}
            onChange={(e) => setDriftTypeFilter(e.target.value)}
            className="px-4 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            <option value="all">All Drift Types</option>
            <option value="modified">Modified</option>
            <option value="shadow">Shadow</option>
            <option value="unmanaged">Unmanaged</option>
          </select>
          <select
            value={severityFilter}
            onChange={(e) => setSeverityFilter(e.target.value)}
            className="px-4 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            <option value="all">All Severities</option>
            <option value="critical">Critical</option>
            <option value="high">High</option>
            <option value="medium">Medium</option>
            <option value="low">Low</option>
          </select>
          <span className="text-sm text-gray-500 ml-auto">
            {filteredResources.length} resource{filteredResources.length !== 1 ? 's' : ''}
          </span>
        </div>
      </div>

      {/* Drifted Resources Table */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50 border-b border-gray-100">
              <tr>
                <th className="w-10 px-4 py-4"></th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Resource</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Type</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Drift Type</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Severity</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Cost Impact</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Description</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Detected</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {filteredResources.map(resource => {
                const isExpanded = expandedRows.has(resource.resource_id)
                return (
                  <tr key={resource.resource_id} className="group">
                    <td colSpan={8} className="p-0">
                      {/* Main row */}
                      <div
                        className="flex items-center cursor-pointer hover:bg-gray-50"
                        onClick={() => toggleRow(resource.resource_id)}
                      >
                        <div className="w-10 px-4 py-4 flex items-center justify-center text-gray-400">
                          {isExpanded ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                        </div>
                        <div className="flex-1 grid grid-cols-7 items-center">
                          <div className="px-6 py-4">
                            <p className="font-medium text-sm">{resource.resource_id}</p>
                            <p className="text-xs text-gray-500">{resource.provider} / {resource.region}</p>
                          </div>
                          <div className="px-6 py-4 text-sm">{resource.resource_type}</div>
                          <div className="px-6 py-4"><DriftTypeBadge type={resource.drift_type} /></div>
                          <div className="px-6 py-4"><SeverityBadge severity={resource.severity} /></div>
                          <div className="px-6 py-4 text-sm font-semibold">
                            {resource.monthly_cost_impact > 0 ? (
                              <span className="text-red-600">${resource.monthly_cost_impact.toLocaleString()}/mo</span>
                            ) : (
                              <span className="text-gray-400">--</span>
                            )}
                          </div>
                          <div className="px-6 py-4 text-sm text-gray-600 truncate max-w-xs" title={resource.description}>
                            {resource.description}
                          </div>
                          <div className="px-6 py-4 text-sm text-gray-500">
                            {new Date(resource.detected_at).toLocaleDateString()}
                          </div>
                        </div>
                      </div>

                      {/* Expanded detail */}
                      {isExpanded && (
                        <div className="px-14 pb-4 bg-gray-50/50 border-t border-gray-100">
                          <div className="flex items-center gap-2 mt-3 mb-1">
                            <Eye size={14} className="text-gray-400" />
                            <span className="text-xs font-semibold text-gray-500 uppercase tracking-wide">
                              State Comparison
                            </span>
                          </div>
                          <StateComparison
                            iacState={resource.iac_state}
                            actualState={resource.actual_state}
                          />
                        </div>
                      )}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
        {filteredResources.length === 0 && (
          <EmptyState
            title="No drifted resources found"
            description="All infrastructure is in sync with IaC definitions, or try adjusting your filters"
            icon={Shield}
          />
        )}
      </div>
    </div>
  )
}

import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Shield, ShieldCheck, ShieldAlert, AlertTriangle, Filter, ToggleLeft, ToggleRight
} from 'lucide-react'
import { api } from '../lib/api'
import { StatCard, SeverityBadge, LoadingSpinner, EmptyState } from '../components/shared'

interface Policy {
  id: string
  name: string
  description: string
  type: string
  enforcement_mode: string
  enabled: boolean
  violation_count: number
  conditions: any
  providers: string[]
  environments: string[]
}

interface PolicyViolation {
  id: string
  policy_name: string
  status: string
  provider: string
  region: string
  resource_id: string
  resource_type: string
  description: string
  severity: string
  details: any
}

interface PolicySummary {
  total_policies: number
  enabled_policies: number
  total_violations: number
  open_violations: number
  by_type: Record<string, number>
  by_severity: Record<string, number>
}

function EnforcementBadge({ mode }: { mode: string }) {
  const colors: Record<string, string> = {
    alert_only: 'bg-blue-100 text-blue-800',
    soft_enforce: 'bg-yellow-100 text-yellow-800',
    hard_enforce: 'bg-red-100 text-red-800',
  }
  const label = mode.replace('_', ' ')
  return (
    <span className={`px-2 py-1 rounded-full text-xs font-medium capitalize ${colors[mode] || 'bg-gray-100 text-gray-800'}`}>
      {label}
    </span>
  )
}

function TypeBadge({ type }: { type: string }) {
  return (
    <span className="px-2 py-1 rounded-full text-xs font-medium bg-purple-100 text-purple-800 capitalize">
      {type.replace('_', ' ')}
    </span>
  )
}

export default function PoliciesPage() {
  const [typeFilter, setTypeFilter] = useState('all')
  const [statusFilter, setStatusFilter] = useState('all')

  const { data: summaryData, isLoading: summaryLoading } = useQuery<PolicySummary>({
    queryKey: ['policies-summary'],
    queryFn: () => api.get('/policies/summary'),
  })

  const { data: policiesData, isLoading: policiesLoading } = useQuery<{ data: Policy[]; total: number }>({
    queryKey: ['policies'],
    queryFn: () => api.get('/policies'),
  })

  const { data: violationsData, isLoading: violationsLoading } = useQuery<{ data: PolicyViolation[]; total: number }>({
    queryKey: ['policies-violations'],
    queryFn: () => api.get('/policies/violations'),
  })

  const isLoading = summaryLoading || policiesLoading || violationsLoading

  const policies = policiesData?.data || []
  const violations = violationsData?.data || []
  const summary = summaryData

  const types = [...new Set(policies.map(p => p.type))]

  const filteredPolicies = policies.filter(p => {
    if (typeFilter !== 'all' && p.type !== typeFilter) return false
    if (statusFilter === 'enabled' && !p.enabled) return false
    if (statusFilter === 'disabled' && p.enabled) return false
    return true
  })

  if (isLoading) return <LoadingSpinner />

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Policy Engine</h1>
        <p className="text-gray-500 text-sm mt-1">Define and enforce cost governance policies across your cloud environments</p>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        <StatCard title="Total Policies" value={String(summary?.total_policies ?? 0)} icon={Shield} />
        <StatCard title="Enabled" value={String(summary?.enabled_policies ?? 0)} icon={ShieldCheck} />
        <StatCard title="Open Violations" value={String(summary?.open_violations ?? 0)} icon={ShieldAlert} />
        <StatCard title="Critical Violations" value={String(summary?.by_severity?.critical ?? 0)} icon={AlertTriangle} />
      </div>

      {/* Policies Section */}
      <div>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold">Policies</h2>
        </div>

        <div className="bg-white rounded-xl shadow-sm p-4 border border-gray-100 mb-4">
          <div className="flex items-center gap-4">
            <Filter size={20} className="text-gray-400" />
            <select value={typeFilter} onChange={(e) => setTypeFilter(e.target.value)}
              className="px-4 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500">
              <option value="all">All Types</option>
              {types.map(t => <option key={t} value={t}>{t.replace('_', ' ')}</option>)}
            </select>
            <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}
              className="px-4 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500">
              <option value="all">All Statuses</option>
              <option value="enabled">Enabled</option>
              <option value="disabled">Disabled</option>
            </select>
            <span className="text-sm text-gray-500 ml-auto">{filteredPolicies.length} policies</span>
          </div>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {filteredPolicies.map(policy => (
            <div key={policy.id} className="bg-white rounded-xl shadow-sm border border-gray-100 p-5">
              <div className="flex items-start justify-between mb-3">
                <div className="flex-1 min-w-0">
                  <h3 className="font-semibold text-sm truncate">{policy.name}</h3>
                  <p className="text-xs text-gray-500 mt-1 line-clamp-2">{policy.description}</p>
                </div>
                <div className="ml-3 flex-shrink-0">
                  {policy.enabled ? (
                    <ToggleRight size={24} className="text-green-500" />
                  ) : (
                    <ToggleLeft size={24} className="text-gray-300" />
                  )}
                </div>
              </div>
              <div className="flex items-center gap-2 flex-wrap">
                <TypeBadge type={policy.type} />
                <EnforcementBadge mode={policy.enforcement_mode} />
                {policy.violation_count > 0 && (
                  <span className="px-2 py-1 rounded-full text-xs font-medium bg-red-50 text-red-700">
                    {policy.violation_count} violation{policy.violation_count !== 1 ? 's' : ''}
                  </span>
                )}
              </div>
            </div>
          ))}
        </div>

        {filteredPolicies.length === 0 && (
          <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
            <EmptyState title="No policies found" description="Try adjusting your filters" icon={Shield} />
          </div>
        )}
      </div>

      {/* Violations Table */}
      <div>
        <h2 className="text-lg font-semibold mb-4">Violations</h2>
        <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead className="bg-gray-50 border-b border-gray-100">
                <tr>
                  <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Resource</th>
                  <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Policy</th>
                  <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Severity</th>
                  <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Status</th>
                  <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Description</th>
                  <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Detected</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {violations.map(violation => (
                  <tr key={violation.id} className="hover:bg-gray-50">
                    <td className="px-6 py-4">
                      <div>
                        <p className="font-medium text-sm">{violation.resource_type}</p>
                        <p className="text-xs text-gray-500 font-mono">{violation.resource_id}</p>
                      </div>
                    </td>
                    <td className="px-6 py-4 text-sm">{violation.policy_name}</td>
                    <td className="px-6 py-4"><SeverityBadge severity={violation.severity} /></td>
                    <td className="px-6 py-4">
                      <span className={`px-2 py-1 rounded-full text-xs font-medium capitalize ${
                        violation.status === 'open' ? 'bg-red-100 text-red-800' :
                        violation.status === 'resolved' ? 'bg-green-100 text-green-800' :
                        'bg-gray-100 text-gray-800'
                      }`}>
                        {violation.status}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-600 max-w-xs truncate">{violation.description}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">
                      {violation.details?.detected_at
                        ? new Date(violation.details.detected_at).toLocaleDateString()
                        : '-'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {violations.length === 0 && (
            <EmptyState title="No violations found" description="All policies are in compliance" icon={ShieldCheck} />
          )}
        </div>
      </div>
    </div>
  )
}

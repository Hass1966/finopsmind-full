import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Clock, CheckCircle, Play, DollarSign, Filter, Shield,
  Plus, Trash2, ToggleLeft, ToggleRight, RotateCcw, X, Ban
} from 'lucide-react'
import { api } from '../lib/api'
import { StatCard, LoadingSpinner, EmptyState } from '../components/shared'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface RemediationAction {
  id: string
  type: string
  status: string
  provider: string
  resource_id: string
  resource_type: string
  description: string
  estimated_savings: number
  currency: string
  risk: string
  auto_approved: boolean
  approval_rule: string
  requested_by: string
  approved_by: string
  approved_at: string | null
  executed_at: string | null
  completed_at: string | null
  failure_reason: string
  audit_log: { timestamp: string; actor: string; action: string; details: string }[]
  created_at: string
}

interface AutoApprovalRule {
  id: string
  name: string
  enabled: boolean
  conditions: {
    max_savings: number
    allowed_types: string[]
    allowed_risks: string[]
    allowed_environments: string[]
  }
  created_by: string
}

interface RemediationSummary {
  total_count: number
  pending_count: number
  completed_count: number
  total_savings_realized: number
  total_savings_pending: number
}

// ---------------------------------------------------------------------------
// Badge helpers
// ---------------------------------------------------------------------------

function RemediationStatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    pending: 'bg-yellow-100 text-yellow-800',
    approved: 'bg-blue-100 text-blue-800',
    executing: 'bg-purple-100 text-purple-800',
    completed: 'bg-green-100 text-green-800',
    failed: 'bg-red-100 text-red-800',
    rolled_back: 'bg-orange-100 text-orange-800',
    rejected: 'bg-gray-100 text-gray-800',
    cancelled: 'bg-gray-100 text-gray-600',
  }
  return (
    <span className={`px-2 py-1 rounded-full text-xs font-medium capitalize ${colors[status] || 'bg-gray-100 text-gray-800'}`}>
      {status.replace('_', ' ')}
    </span>
  )
}

function RiskBadge({ risk }: { risk: string }) {
  const colors: Record<string, string> = {
    low: 'bg-green-100 text-green-800',
    medium: 'bg-yellow-100 text-yellow-800',
    high: 'bg-orange-100 text-orange-800',
    critical: 'bg-red-100 text-red-800',
  }
  return (
    <span className={`px-2 py-1 rounded-full text-xs font-medium capitalize ${colors[risk] || 'bg-gray-100 text-gray-800'}`}>
      {risk}
    </span>
  )
}

// ---------------------------------------------------------------------------
// New Rule Form
// ---------------------------------------------------------------------------

function NewRuleForm({ onSubmit, onCancel }: {
  onSubmit: (rule: Omit<AutoApprovalRule, 'id' | 'created_by'>) => void
  onCancel: () => void
}) {
  const [name, setName] = useState('')
  const [maxSavings, setMaxSavings] = useState(500)
  const [allowedTypes, setAllowedTypes] = useState('')
  const [allowedRisks, setAllowedRisks] = useState('low')
  const [allowedEnvs, setAllowedEnvs] = useState('dev,staging')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSubmit({
      name,
      enabled: true,
      conditions: {
        max_savings: maxSavings,
        allowed_types: allowedTypes.split(',').map(s => s.trim()).filter(Boolean),
        allowed_risks: allowedRisks.split(',').map(s => s.trim()).filter(Boolean),
        allowed_environments: allowedEnvs.split(',').map(s => s.trim()).filter(Boolean),
      },
    })
  }

  return (
    <form onSubmit={handleSubmit} className="bg-gray-50 rounded-lg p-4 border border-gray-200 space-y-4">
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Rule Name</label>
          <input type="text" value={name} onChange={(e) => setName(e.target.value)} required
            className="w-full px-3 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            placeholder="e.g. Auto-approve low-risk dev cleanup" />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Max Savings Threshold ($)</label>
          <input type="number" value={maxSavings} onChange={(e) => setMaxSavings(Number(e.target.value))} required
            className="w-full px-3 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Allowed Types (comma-separated)</label>
          <input type="text" value={allowedTypes} onChange={(e) => setAllowedTypes(e.target.value)}
            className="w-full px-3 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            placeholder="e.g. rightsizing,unused_resource" />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Allowed Risk Levels (comma-separated)</label>
          <input type="text" value={allowedRisks} onChange={(e) => setAllowedRisks(e.target.value)}
            className="w-full px-3 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            placeholder="e.g. low,medium" />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Allowed Environments (comma-separated)</label>
          <input type="text" value={allowedEnvs} onChange={(e) => setAllowedEnvs(e.target.value)}
            className="w-full px-3 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            placeholder="e.g. dev,staging" />
        </div>
      </div>
      <div className="flex gap-2 justify-end">
        <button type="button" onClick={onCancel}
          className="px-4 py-2 text-sm text-gray-600 bg-white border border-gray-200 rounded-lg hover:bg-gray-50">
          Cancel
        </button>
        <button type="submit"
          className="px-4 py-2 text-sm text-white bg-blue-600 rounded-lg hover:bg-blue-700">
          Create Rule
        </button>
      </div>
    </form>
  )
}

// ---------------------------------------------------------------------------
// Main Page
// ---------------------------------------------------------------------------

export default function RemediationPage() {
  const queryClient = useQueryClient()
  const [statusFilter, setStatusFilter] = useState('all')
  const [typeFilter, setTypeFilter] = useState('all')
  const [showNewRule, setShowNewRule] = useState(false)
  const [rejectingId, setRejectingId] = useState<string | null>(null)
  const [rejectReason, setRejectReason] = useState('')

  // ---- Queries ----

  const { data: actionsData, isLoading: actionsLoading } = useQuery<{ data: RemediationAction[]; total: number }>({
    queryKey: ['remediations'],
    queryFn: () => api.get('/remediations?page_size=50'),
  })

  const { data: summaryData, isLoading: summaryLoading } = useQuery<RemediationSummary>({
    queryKey: ['remediations', 'summary'],
    queryFn: () => api.get('/remediations/summary'),
  })

  const { data: rulesData } = useQuery<{ data: AutoApprovalRule[] }>({
    queryKey: ['remediations', 'rules'],
    queryFn: () => api.get('/remediations/rules'),
  })

  // ---- Mutations ----

  const approveMutation = useMutation({
    mutationFn: (id: string) => api.post(`/remediations/${id}/approve`, {}),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['remediations'] })
    },
  })

  const rejectMutation = useMutation({
    mutationFn: ({ id, reason }: { id: string; reason: string }) =>
      api.post(`/remediations/${id}/reject`, { reason }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['remediations'] })
      setRejectingId(null)
      setRejectReason('')
    },
  })

  const cancelMutation = useMutation({
    mutationFn: (id: string) => api.post(`/remediations/${id}/cancel`, {}),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['remediations'] })
    },
  })

  const rollbackMutation = useMutation({
    mutationFn: (id: string) => api.post(`/remediations/${id}/rollback`, {}),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['remediations'] })
    },
  })

  const createRuleMutation = useMutation({
    mutationFn: (rule: Omit<AutoApprovalRule, 'id' | 'created_by'>) =>
      api.post('/remediations/rules', rule),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['remediations', 'rules'] })
      setShowNewRule(false)
    },
  })

  const deleteRuleMutation = useMutation({
    mutationFn: (id: string) => api.delete(`/remediations/rules/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['remediations', 'rules'] })
    },
  })

  const toggleRuleMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      api.post('/remediations/rules', { id, enabled }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['remediations', 'rules'] })
    },
  })

  // ---- Derived data ----

  const actions = actionsData?.data || []
  const rules = rulesData?.data || []
  const types = [...new Set(actions.map(a => a.type))]

  const inProgressCount = actions.filter(a => ['approved', 'executing'].includes(a.status)).length

  const filteredActions = actions.filter(a => {
    if (statusFilter !== 'all' && a.status !== statusFilter) return false
    if (typeFilter !== 'all' && a.type !== typeFilter) return false
    return true
  })

  // ---- Loading ----

  if (actionsLoading || summaryLoading) return <LoadingSpinner />

  const summary = summaryData || {
    total_count: 0,
    pending_count: 0,
    completed_count: 0,
    total_savings_realized: 0,
    total_savings_pending: 0,
  }

  // ---- Render ----

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold">Automated Remediation</h1>
        <p className="text-gray-500 text-sm mt-1">
          Review, approve, and track automated cost optimization actions
        </p>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        <StatCard
          title="Pending Approval"
          value={String(summary.pending_count)}
          icon={Clock}
        />
        <StatCard
          title="In Progress"
          value={String(inProgressCount)}
          icon={Play}
        />
        <StatCard
          title="Completed"
          value={String(summary.completed_count)}
          icon={CheckCircle}
        />
        <StatCard
          title="Total Savings"
          value={`$${summary.total_savings_realized.toLocaleString()}`}
          subtitle={`$${summary.total_savings_pending.toLocaleString()} pending`}
          icon={DollarSign}
        />
      </div>

      {/* Filters */}
      <div className="bg-white rounded-xl shadow-sm p-4 border border-gray-100">
        <div className="flex items-center gap-4">
          <Filter size={20} className="text-gray-400" />
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="px-4 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            <option value="all">All Statuses</option>
            <option value="pending">Pending</option>
            <option value="approved">Approved</option>
            <option value="executing">Executing</option>
            <option value="completed">Completed</option>
            <option value="failed">Failed</option>
            <option value="rolled_back">Rolled Back</option>
            <option value="rejected">Rejected</option>
            <option value="cancelled">Cancelled</option>
          </select>
          <select
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value)}
            className="px-4 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            <option value="all">All Types</option>
            {types.map(t => (
              <option key={t} value={t}>{t.replace(/_/g, ' ')}</option>
            ))}
          </select>
          <span className="text-sm text-gray-500 ml-auto">
            {filteredActions.length} action{filteredActions.length !== 1 ? 's' : ''}
          </span>
        </div>
      </div>

      {/* Actions Table */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50 border-b border-gray-100">
              <tr>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Resource</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Type</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Status</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Risk</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Savings</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Requested By</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Date</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {filteredActions.map(action => (
                <tr key={action.id} className="hover:bg-gray-50">
                  <td className="px-6 py-4">
                    <div>
                      <p className="font-medium text-sm">{action.resource_type}</p>
                      <p className="text-xs text-gray-500 font-mono">{action.resource_id}</p>
                      <p className="text-xs text-gray-400 mt-0.5">{action.provider}</p>
                    </div>
                  </td>
                  <td className="px-6 py-4 text-sm capitalize">{action.type.replace(/_/g, ' ')}</td>
                  <td className="px-6 py-4">
                    <RemediationStatusBadge status={action.status} />
                    {action.auto_approved && (
                      <span className="ml-1 text-xs text-blue-500" title={`Rule: ${action.approval_rule}`}>auto</span>
                    )}
                  </td>
                  <td className="px-6 py-4"><RiskBadge risk={action.risk} /></td>
                  <td className="px-6 py-4 text-sm text-green-600 font-semibold">
                    ${action.estimated_savings.toLocaleString()}/{action.currency === 'USD' ? 'mo' : action.currency}
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-600">{action.requested_by}</td>
                  <td className="px-6 py-4 text-sm text-gray-500">
                    {new Date(action.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex flex-wrap gap-1">
                      {/* Pending: approve / reject */}
                      {action.status === 'pending' && (
                        <>
                          <button
                            onClick={() => approveMutation.mutate(action.id)}
                            disabled={approveMutation.isPending}
                            className="text-xs px-2 py-1 bg-green-100 text-green-800 rounded hover:bg-green-200 flex items-center gap-1"
                          >
                            <CheckCircle size={12} /> Approve
                          </button>
                          <button
                            onClick={() => setRejectingId(action.id)}
                            className="text-xs px-2 py-1 bg-red-100 text-red-800 rounded hover:bg-red-200 flex items-center gap-1"
                          >
                            <X size={12} /> Reject
                          </button>
                        </>
                      )}

                      {/* Approved / Executing: cancel */}
                      {['approved', 'executing'].includes(action.status) && (
                        <button
                          onClick={() => cancelMutation.mutate(action.id)}
                          disabled={cancelMutation.isPending}
                          className="text-xs px-2 py-1 bg-gray-100 text-gray-800 rounded hover:bg-gray-200 flex items-center gap-1"
                        >
                          <Ban size={12} /> Cancel
                        </button>
                      )}

                      {/* Completed / Failed: rollback */}
                      {['completed', 'failed'].includes(action.status) && (
                        <button
                          onClick={() => rollbackMutation.mutate(action.id)}
                          disabled={rollbackMutation.isPending}
                          className="text-xs px-2 py-1 bg-orange-100 text-orange-800 rounded hover:bg-orange-200 flex items-center gap-1"
                        >
                          <RotateCcw size={12} /> Rollback
                        </button>
                      )}

                      {/* Failed: show reason tooltip */}
                      {action.status === 'failed' && action.failure_reason && (
                        <span className="text-xs text-red-500 italic" title={action.failure_reason}>
                          failed
                        </span>
                      )}
                    </div>

                    {/* Inline reject reason form */}
                    {rejectingId === action.id && (
                      <div className="mt-2 flex gap-1">
                        <input
                          type="text"
                          value={rejectReason}
                          onChange={(e) => setRejectReason(e.target.value)}
                          placeholder="Rejection reason..."
                          className="text-xs px-2 py-1 border border-gray-200 rounded flex-1 focus:outline-none focus:ring-1 focus:ring-red-400"
                        />
                        <button
                          onClick={() => rejectMutation.mutate({ id: action.id, reason: rejectReason })}
                          disabled={rejectMutation.isPending || !rejectReason.trim()}
                          className="text-xs px-2 py-1 bg-red-600 text-white rounded hover:bg-red-700 disabled:opacity-50"
                        >
                          Confirm
                        </button>
                        <button
                          onClick={() => { setRejectingId(null); setRejectReason('') }}
                          className="text-xs px-2 py-1 bg-gray-100 text-gray-600 rounded hover:bg-gray-200"
                        >
                          Cancel
                        </button>
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {filteredActions.length === 0 && (
          <EmptyState title="No remediation actions found" description="Try adjusting your filters" icon={Shield} />
        )}
      </div>

      {/* Auto-Approval Rules */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-100 p-6 space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-lg font-semibold">Auto-Approval Rules</h2>
            <p className="text-sm text-gray-500 mt-0.5">
              Define rules to automatically approve low-risk remediation actions
            </p>
          </div>
          <button
            onClick={() => setShowNewRule(!showNewRule)}
            className="flex items-center gap-1 px-4 py-2 text-sm text-white bg-blue-600 rounded-lg hover:bg-blue-700"
          >
            <Plus size={16} /> New Rule
          </button>
        </div>

        {showNewRule && (
          <NewRuleForm
            onSubmit={(rule) => createRuleMutation.mutate(rule)}
            onCancel={() => setShowNewRule(false)}
          />
        )}

        {rules.length === 0 && !showNewRule && (
          <p className="text-sm text-gray-400 py-4 text-center">
            No auto-approval rules configured yet.
          </p>
        )}

        <div className="space-y-3">
          {rules.map(rule => (
            <div
              key={rule.id}
              className={`flex items-center justify-between p-4 rounded-lg border ${
                rule.enabled ? 'border-green-200 bg-green-50/50' : 'border-gray-200 bg-gray-50'
              }`}
            >
              <div className="flex-1">
                <div className="flex items-center gap-2">
                  <span className="font-medium text-sm">{rule.name}</span>
                  <span className={`text-xs px-1.5 py-0.5 rounded ${
                    rule.enabled ? 'bg-green-100 text-green-700' : 'bg-gray-200 text-gray-500'
                  }`}>
                    {rule.enabled ? 'Active' : 'Disabled'}
                  </span>
                </div>
                <div className="flex flex-wrap gap-3 mt-1 text-xs text-gray-500">
                  <span>Max savings: ${rule.conditions.max_savings.toLocaleString()}</span>
                  {rule.conditions.allowed_risks.length > 0 && (
                    <span>Risks: {rule.conditions.allowed_risks.join(', ')}</span>
                  )}
                  {rule.conditions.allowed_types.length > 0 && (
                    <span>Types: {rule.conditions.allowed_types.join(', ')}</span>
                  )}
                  {rule.conditions.allowed_environments.length > 0 && (
                    <span>Envs: {rule.conditions.allowed_environments.join(', ')}</span>
                  )}
                  <span>Created by {rule.created_by}</span>
                </div>
              </div>
              <div className="flex items-center gap-2 ml-4">
                <button
                  onClick={() => toggleRuleMutation.mutate({ id: rule.id, enabled: !rule.enabled })}
                  className="p-1.5 rounded hover:bg-white/80 text-gray-500 hover:text-gray-700"
                  title={rule.enabled ? 'Disable rule' : 'Enable rule'}
                >
                  {rule.enabled ? <ToggleRight size={20} className="text-green-600" /> : <ToggleLeft size={20} />}
                </button>
                <button
                  onClick={() => deleteRuleMutation.mutate(rule.id)}
                  disabled={deleteRuleMutation.isPending}
                  className="p-1.5 rounded hover:bg-red-50 text-gray-400 hover:text-red-600"
                  title="Delete rule"
                >
                  <Trash2 size={16} />
                </button>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

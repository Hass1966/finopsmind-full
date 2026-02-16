import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  AlertTriangle, AlertCircle, Clock, CheckCircle, Filter
} from 'lucide-react'
import { api } from '../lib/api'
import type { Anomaly } from '../lib/types'
import { StatCard, SeverityBadge, StatusBadge, LoadingSpinner, EmptyState } from '../components/shared'

export default function AnomaliesPage() {
  const queryClient = useQueryClient()
  const [statusFilter, setStatusFilter] = useState('all')
  const [severityFilter, setSeverityFilter] = useState('all')

  const { data: anomaliesData, isLoading } = useQuery<{ data: Anomaly[] }>({
    queryKey: ['anomalies'],
    queryFn: () => api.get('/anomalies?page_size=50')
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, status }: { id: string; status: string }) =>
      api.patch(`/anomalies/${id}`, { status }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['anomalies'] })
  })

  const anomalies = anomaliesData?.data || []

  const filteredAnomalies = anomalies.filter(a => {
    if (statusFilter !== 'all' && a.status !== statusFilter) return false
    if (severityFilter !== 'all' && a.severity !== severityFilter) return false
    return true
  })

  const stats = {
    total: anomalies.length,
    critical: anomalies.filter(a => a.severity === 'critical').length,
    open: anomalies.filter(a => a.status === 'open').length,
    resolved: anomalies.filter(a => a.status === 'resolved').length,
  }

  if (isLoading) return <LoadingSpinner />

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Anomaly Detection</h1>
        <p className="text-gray-500 text-sm mt-1">AI-detected cost anomalies and unusual spending patterns</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        <StatCard title="Total Anomalies" value={String(stats.total)} icon={AlertTriangle} />
        <StatCard title="Critical" value={String(stats.critical)} icon={AlertCircle} />
        <StatCard title="Open" value={String(stats.open)} icon={Clock} />
        <StatCard title="Resolved" value={String(stats.resolved)} icon={CheckCircle} />
      </div>

      <div className="bg-white rounded-xl shadow-sm p-4 border border-gray-100">
        <div className="flex items-center gap-4">
          <Filter size={20} className="text-gray-400" />
          <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}
            className="px-4 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500">
            <option value="all">All Statuses</option>
            <option value="open">Open</option>
            <option value="investigating">Investigating</option>
            <option value="resolved">Resolved</option>
          </select>
          <select value={severityFilter} onChange={(e) => setSeverityFilter(e.target.value)}
            className="px-4 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500">
            <option value="all">All Severities</option>
            <option value="critical">Critical</option>
            <option value="high">High</option>
            <option value="medium">Medium</option>
            <option value="low">Low</option>
          </select>
          <span className="text-sm text-gray-500 ml-auto">{filteredAnomalies.length} anomalies</span>
        </div>
      </div>

      <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50 border-b border-gray-100">
              <tr>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Date</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Service</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Expected</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Actual</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Deviation</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Severity</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Status</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {filteredAnomalies.map(anomaly => (
                <tr key={anomaly.id} className="hover:bg-gray-50">
                  <td className="px-6 py-4 text-sm">{new Date(anomaly.date).toLocaleDateString()}</td>
                  <td className="px-6 py-4">
                    <div>
                      <p className="font-medium text-sm">{anomaly.service}</p>
                      <p className="text-xs text-gray-500">{anomaly.provider}</p>
                    </div>
                  </td>
                  <td className="px-6 py-4 text-sm">${anomaly.expected_amount?.toLocaleString()}</td>
                  <td className="px-6 py-4 text-sm font-medium text-red-600">${anomaly.actual_amount?.toLocaleString()}</td>
                  <td className="px-6 py-4">
                    <span className={`text-sm font-medium ${anomaly.deviation_pct > 0 ? 'text-red-600' : 'text-green-600'}`}>
                      {anomaly.deviation_pct > 0 ? '+' : ''}{anomaly.deviation_pct?.toFixed(1)}%
                    </span>
                  </td>
                  <td className="px-6 py-4"><SeverityBadge severity={anomaly.severity} /></td>
                  <td className="px-6 py-4"><StatusBadge status={anomaly.status} /></td>
                  <td className="px-6 py-4">
                    {anomaly.status !== 'resolved' && (
                      <div className="flex gap-2">
                        {anomaly.status === 'open' && (
                          <button onClick={() => updateMutation.mutate({ id: anomaly.id, status: 'investigating' })}
                            className="text-xs px-2 py-1 bg-yellow-100 text-yellow-800 rounded hover:bg-yellow-200">Investigate</button>
                        )}
                        <button onClick={() => updateMutation.mutate({ id: anomaly.id, status: 'resolved' })}
                          className="text-xs px-2 py-1 bg-green-100 text-green-800 rounded hover:bg-green-200">Resolve</button>
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {filteredAnomalies.length === 0 && (
          <EmptyState title="No anomalies found" description="Try adjusting your filters" icon={CheckCircle} />
        )}
      </div>
    </div>
  )
}

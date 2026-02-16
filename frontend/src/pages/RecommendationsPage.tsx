import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  DollarSign, Lightbulb, ArrowUpRight, CheckCircle, Check, Filter
} from 'lucide-react'
import {
  XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, BarChart, Bar
} from 'recharts'
import { api } from '../lib/api'
import type { Recommendation } from '../lib/types'
import { StatCard, ImpactBadge, StatusBadge, LoadingSpinner, EmptyState } from '../components/shared'

export default function RecommendationsPage() {
  const queryClient = useQueryClient()
  const [typeFilter, setTypeFilter] = useState('all')
  const [impactFilter, setImpactFilter] = useState('all')

  const { data: recommendationsData, isLoading } = useQuery<{ data: Recommendation[] }>({
    queryKey: ['recommendations'],
    queryFn: () => api.get('/recommendations?page_size=50')
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, status }: { id: string; status: string }) =>
      api.patch(`/recommendations/${id}`, { status }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['recommendations'] })
  })

  const recommendations = recommendationsData?.data || []

  const filteredRecs = recommendations.filter(r => {
    if (typeFilter !== 'all' && r.type !== typeFilter) return false
    if (impactFilter !== 'all' && r.impact !== impactFilter) return false
    return true
  })

  const stats = {
    total: recommendations.length,
    totalSavings: recommendations.reduce((sum, r) => sum + r.estimated_savings, 0),
    highImpact: recommendations.filter(r => r.impact === 'high').length,
    implemented: recommendations.filter(r => r.status === 'implemented').length,
  }

  const types = [...new Set(recommendations.map(r => r.type))]

  if (isLoading) return <LoadingSpinner />

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Cost Optimization</h1>
        <p className="text-gray-500 text-sm mt-1">AI-powered recommendations to reduce cloud spending</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        <StatCard title="Total Recommendations" value={String(stats.total)} icon={Lightbulb} />
        <StatCard title="Potential Savings" value={`$${stats.totalSavings.toLocaleString()}/mo`} icon={DollarSign} />
        <StatCard title="High Impact" value={String(stats.highImpact)} icon={ArrowUpRight} />
        <StatCard title="Implemented" value={String(stats.implemented)} icon={CheckCircle} />
      </div>

      <div className="bg-white rounded-xl shadow-sm p-6 border border-gray-100">
        <h2 className="text-lg font-semibold mb-4">Savings by Recommendation Type</h2>
        <ResponsiveContainer width="100%" height={300}>
          <BarChart data={types.map(type => ({
            type: type.replace('_', ' '),
            savings: recommendations.filter(r => r.type === type).reduce((sum, r) => sum + r.estimated_savings, 0),
            count: recommendations.filter(r => r.type === type).length,
          }))}>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey="type" tick={{ fontSize: 12 }} />
            <YAxis tick={{ fontSize: 12 }} tickFormatter={(v) => `$${v}`} />
            <Tooltip formatter={(v: number) => `$${v.toLocaleString()}`} />
            <Bar dataKey="savings" fill="#10b981" radius={[4, 4, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </div>

      <div className="bg-white rounded-xl shadow-sm p-4 border border-gray-100">
        <div className="flex items-center gap-4">
          <Filter size={20} className="text-gray-400" />
          <select value={typeFilter} onChange={(e) => setTypeFilter(e.target.value)}
            className="px-4 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500">
            <option value="all">All Types</option>
            {types.map(t => <option key={t} value={t}>{t.replace('_', ' ')}</option>)}
          </select>
          <select value={impactFilter} onChange={(e) => setImpactFilter(e.target.value)}
            className="px-4 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500">
            <option value="all">All Impact Levels</option>
            <option value="high">High</option>
            <option value="medium">Medium</option>
            <option value="low">Low</option>
          </select>
          <span className="text-sm text-gray-500 ml-auto">{filteredRecs.length} recommendations</span>
        </div>
      </div>

      <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50 border-b border-gray-100">
              <tr>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Resource</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Type</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Current</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Recommended</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Savings</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Impact</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Status</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {filteredRecs.map(rec => (
                <tr key={rec.id} className="hover:bg-gray-50">
                  <td className="px-6 py-4">
                    <div>
                      <p className="font-medium text-sm">{rec.resource_type}</p>
                      <p className="text-xs text-gray-500 font-mono">{rec.resource_id}</p>
                    </div>
                  </td>
                  <td className="px-6 py-4 text-sm capitalize">{rec.type?.replace('_', ' ')}</td>
                  <td className="px-6 py-4 text-sm text-gray-600">{rec.current_config}</td>
                  <td className="px-6 py-4 text-sm text-blue-600 font-medium">{rec.recommended_config}</td>
                  <td className="px-6 py-4 text-sm text-green-600 font-semibold">${rec.estimated_savings?.toLocaleString()}/mo</td>
                  <td className="px-6 py-4"><ImpactBadge impact={rec.impact} /></td>
                  <td className="px-6 py-4"><StatusBadge status={rec.status} /></td>
                  <td className="px-6 py-4">
                    {rec.status === 'open' && (
                      <div className="flex gap-2">
                        <button onClick={() => updateMutation.mutate({ id: rec.id, status: 'implemented' })}
                          className="text-xs px-2 py-1 bg-green-100 text-green-800 rounded hover:bg-green-200 flex items-center gap-1">
                          <Check size={12} /> Implement
                        </button>
                        <button onClick={() => updateMutation.mutate({ id: rec.id, status: 'dismissed' })}
                          className="text-xs px-2 py-1 bg-gray-100 text-gray-800 rounded hover:bg-gray-200">Dismiss</button>
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {filteredRecs.length === 0 && (
          <EmptyState title="No recommendations found" description="Try adjusting your filters" icon={Lightbulb} />
        )}
      </div>
    </div>
  )
}

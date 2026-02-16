import { useQuery } from '@tanstack/react-query'
import {
  DollarSign, Calendar, TrendingUp, PieChart, AlertTriangle, Lightbulb,
  ArrowUpRight, ArrowDownRight, ShoppingCart
} from 'lucide-react'
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Cell
} from 'recharts'
import { api } from '../lib/api'
import { StatCard, LoadingSpinner, EmptyState } from '../components/shared'

interface Commitment {
  id: string
  type: string
  service: string
  instance_type: string
  quantity: number
  term: string
  payment: string
  monthly_cost: number
  on_demand_cost: number
  savings: number
  utilization: number
  start_date: string
  end_date: string
  status: string
  days_remaining: number
}

interface CommitmentRecommendation {
  action: string
  type: string
  savings_impact: number
  priority: string
}

interface PortfolioSummary {
  total_commitments: number
  total_monthly_commit: number
  total_monthly_savings: number
  avg_utilization: number
  expiring_soon: number
  underutilized: number
}

interface PortfolioResponse {
  summary: PortfolioSummary
  commitments: Commitment[]
  recommendations: CommitmentRecommendation[]
}

function UtilizationBar({ value }: { value: number }) {
  const color =
    value >= 80 ? 'bg-green-500' :
    value >= 60 ? 'bg-yellow-500' :
    'bg-red-500'

  return (
    <div className="flex items-center gap-2">
      <div className="flex-1 h-2 bg-gray-200 rounded-full overflow-hidden w-24">
        <div
          className={`h-full rounded-full ${color}`}
          style={{ width: `${Math.min(value, 100)}%` }}
        />
      </div>
      <span className="text-xs font-medium text-gray-700 w-10 text-right">{value}%</span>
    </div>
  )
}

function CommitmentStatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    active: 'bg-green-100 text-green-800',
    expiring: 'bg-orange-100 text-orange-800',
    underutilized: 'bg-red-100 text-red-800',
    expired: 'bg-gray-100 text-gray-800',
  }
  return (
    <span className={`px-2 py-1 rounded-full text-xs font-medium capitalize ${colors[status] || 'bg-gray-100 text-gray-800'}`}>
      {status}
    </span>
  )
}

function PriorityBadge({ priority }: { priority: string }) {
  const colors: Record<string, string> = {
    high: 'bg-red-100 text-red-800',
    medium: 'bg-yellow-100 text-yellow-800',
    low: 'bg-blue-100 text-blue-800',
  }
  return (
    <span className={`px-2 py-1 rounded-full text-xs font-medium capitalize ${colors[priority] || 'bg-gray-100 text-gray-800'}`}>
      {priority}
    </span>
  )
}

function RecommendationIcon({ type }: { type: string }) {
  switch (type) {
    case 'renewal': return <Calendar size={16} className="text-orange-500" />
    case 'convert': return <ArrowDownRight size={16} className="text-red-500" />
    case 'purchase': return <ShoppingCart size={16} className="text-blue-500" />
    default: return <Lightbulb size={16} className="text-yellow-500" />
  }
}

export default function CommitmentsPage() {
  const { data, isLoading } = useQuery<PortfolioResponse>({
    queryKey: ['commitments-portfolio'],
    queryFn: () => api.get('/commitments/portfolio'),
  })

  if (isLoading) return <LoadingSpinner />

  const summary = data?.summary
  const commitments = data?.commitments || []
  const recommendations = data?.recommendations || []

  const expiringSoon = commitments.filter(c => c.days_remaining <= 30 && c.days_remaining > 0)

  const utilizationChartData = commitments.map(c => ({
    name: `${c.service} (${c.id})`,
    utilization: c.utilization,
    service: c.service,
  }))

  const UTIL_COLORS = (val: number) =>
    val >= 80 ? '#22c55e' : val >= 60 ? '#eab308' : '#ef4444'

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Commitment Portfolio</h1>
        <p className="text-gray-500 text-sm mt-1">
          Manage Reserved Instances and Savings Plans across your cloud accounts
        </p>
      </div>

      {/* Expiring Soon Alerts */}
      {expiringSoon.length > 0 && (
        <div className="bg-orange-50 border border-orange-200 rounded-xl p-4">
          <div className="flex items-center gap-2 mb-2">
            <AlertTriangle size={18} className="text-orange-600" />
            <h3 className="font-semibold text-orange-800 text-sm">
              {expiringSoon.length} commitment{expiringSoon.length !== 1 ? 's' : ''} expiring soon
            </h3>
          </div>
          <div className="space-y-1">
            {expiringSoon.map(c => (
              <p key={c.id} className="text-sm text-orange-700">
                <span className="font-medium">{c.id}</span> - {c.type} ({c.service} {c.instance_type})
                expires in <span className="font-semibold">{c.days_remaining} days</span> on {c.end_date}
              </p>
            ))}
          </div>
        </div>
      )}

      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        <StatCard
          title="Total Commitments"
          value={String(summary?.total_commitments ?? 0)}
          icon={PieChart}
        />
        <StatCard
          title="Monthly Commit"
          value={`$${(summary?.total_monthly_commit ?? 0).toLocaleString()}`}
          icon={DollarSign}
        />
        <StatCard
          title="Monthly Savings"
          value={`$${(summary?.total_monthly_savings ?? 0).toLocaleString()}`}
          subtitle={summary ? `${((summary.total_monthly_savings / (summary.total_monthly_commit + summary.total_monthly_savings)) * 100).toFixed(0)}% effective discount` : undefined}
          icon={TrendingUp}
        />
        <StatCard
          title="Avg Utilization"
          value={`${summary?.avg_utilization ?? 0}%`}
          subtitle={summary?.underutilized ? `${summary.underutilized} underutilized` : undefined}
          icon={ArrowUpRight}
        />
      </div>

      {/* Utilization Chart */}
      <div className="bg-white rounded-xl shadow-sm p-6 border border-gray-100">
        <h2 className="text-lg font-semibold mb-4">Utilization by Commitment</h2>
        <ResponsiveContainer width="100%" height={280}>
          <BarChart data={utilizationChartData} layout="vertical" margin={{ left: 40 }}>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis type="number" domain={[0, 100]} tick={{ fontSize: 12 }} tickFormatter={v => `${v}%`} />
            <YAxis type="category" dataKey="name" tick={{ fontSize: 11 }} width={140} />
            <Tooltip formatter={(v: number) => `${v}%`} />
            <Bar dataKey="utilization" radius={[0, 4, 4, 0]}>
              {utilizationChartData.map((entry, idx) => (
                <Cell key={idx} fill={UTIL_COLORS(entry.utilization)} />
              ))}
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      </div>

      {/* Commitments Table */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-100">
          <h2 className="text-lg font-semibold">All Commitments</h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50 border-b border-gray-100">
              <tr>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Type</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Service</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Instance Type</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Term</th>
                <th className="text-right px-6 py-4 text-sm font-medium text-gray-500">Monthly Cost</th>
                <th className="text-right px-6 py-4 text-sm font-medium text-gray-500">Savings</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Utilization</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Expiry</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Status</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {commitments.map(c => (
                <tr key={c.id} className="hover:bg-gray-50">
                  <td className="px-6 py-4">
                    <div>
                      <p className="font-medium text-sm">{c.type}</p>
                      <p className="text-xs text-gray-400 font-mono">{c.id}</p>
                    </div>
                  </td>
                  <td className="px-6 py-4 text-sm">{c.service}</td>
                  <td className="px-6 py-4 text-sm font-mono text-gray-600">{c.instance_type}</td>
                  <td className="px-6 py-4">
                    <div>
                      <p className="text-sm">{c.term}</p>
                      <p className="text-xs text-gray-400">{c.payment}</p>
                    </div>
                  </td>
                  <td className="px-6 py-4 text-sm text-right font-medium">${c.monthly_cost.toLocaleString()}/mo</td>
                  <td className="px-6 py-4 text-sm text-right text-green-600 font-semibold">
                    ${c.savings.toLocaleString()}/mo
                  </td>
                  <td className="px-6 py-4 w-40">
                    <UtilizationBar value={c.utilization} />
                  </td>
                  <td className="px-6 py-4">
                    <div>
                      <p className="text-sm">{c.end_date}</p>
                      <p className={`text-xs ${c.days_remaining <= 30 ? 'text-orange-600 font-semibold' : 'text-gray-400'}`}>
                        {c.days_remaining}d remaining
                      </p>
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    <CommitmentStatusBadge status={c.status} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {commitments.length === 0 && (
          <EmptyState
            title="No commitments found"
            description="No Reserved Instances or Savings Plans detected"
            icon={PieChart}
          />
        )}
      </div>

      {/* Recommendations */}
      {recommendations.length > 0 && (
        <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
          <div className="px-6 py-4 border-b border-gray-100">
            <h2 className="text-lg font-semibold">Recommendations</h2>
            <p className="text-sm text-gray-500 mt-1">Actions to optimize your commitment portfolio</p>
          </div>
          <div className="divide-y divide-gray-100">
            {recommendations.map((rec, idx) => (
              <div key={idx} className="px-6 py-4 flex items-center gap-4 hover:bg-gray-50">
                <div className="p-2 bg-gray-50 rounded-lg">
                  <RecommendationIcon type={rec.type} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-gray-900">{rec.action}</p>
                  <div className="flex items-center gap-3 mt-1">
                    <span className={`text-xs font-medium ${
                      rec.savings_impact >= 0 ? 'text-green-600' : 'text-red-600'
                    }`}>
                      {rec.savings_impact >= 0 ? '+' : ''}${rec.savings_impact.toLocaleString()}/mo
                    </span>
                    <span className="text-xs text-gray-400 capitalize">{rec.type}</span>
                  </div>
                </div>
                <PriorityBadge priority={rec.priority} />
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

import { useQuery } from '@tanstack/react-query'
import {
  DollarSign, AlertTriangle, TrendingUp, Lightbulb
} from 'lucide-react'
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
  PieChart, Pie, Cell
} from 'recharts'
import { api } from '../lib/api'
import type { CostSummary, CostTrend, Anomaly, Budget, Recommendation, Provider } from '../lib/types'
import { StatCard, SeverityBadge, LoadingSpinner } from '../components/shared'

const COLORS = ['#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6', '#ec4899']

export default function DashboardPage() {
  const { data: summary, isLoading: loadingSummary } = useQuery<CostSummary>({ queryKey: ['costSummary'], queryFn: () => api.get('/costs/summary') })
  const { data: trend } = useQuery<CostTrend>({ queryKey: ['costTrend'], queryFn: () => api.get('/costs/trend') })
  const { data: anomaliesData } = useQuery<{ data: Anomaly[] }>({ queryKey: ['anomalies'], queryFn: () => api.get('/anomalies?page_size=5') })
  const { data: budgets } = useQuery<{ data: Budget[] }>({ queryKey: ['budgets'], queryFn: () => api.get('/budgets') })
  const { data: recommendations } = useQuery<{ data: Recommendation[] }>({ queryKey: ['recommendations'], queryFn: () => api.get('/recommendations') })
  const { data: providers } = useQuery<Provider[]>({ queryKey: ['providers'], queryFn: () => api.get('/providers') })

  const chartData = trend?.data_points?.map(d => ({ date: new Date(d.date).toLocaleDateString('en-US', { month: 'short', day: 'numeric' }), cost: d.total })) || []
  const pieData = summary?.by_service?.slice(0, 6).map(s => ({ name: s.name, value: s.amount })) || []

  const hasProviders = providers && providers.length > 0
  const hasData = (summary?.total_cost || 0) > 0

  if (loadingSummary) return <LoadingSpinner />

  // Onboarding: no providers connected
  if (!hasProviders) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-bold">Dashboard</h1>
        <div className="bg-white rounded-xl shadow-sm border border-gray-100 p-12 text-center">
          <div className="w-16 h-16 bg-blue-100 rounded-full flex items-center justify-center mx-auto mb-4">
            <DollarSign className="w-8 h-8 text-blue-600" />
          </div>
          <h2 className="text-xl font-semibold mb-2">Welcome to FinOpsMind</h2>
          <p className="text-gray-500 mb-6 max-w-md mx-auto">
            Connect your cloud provider to start tracking costs, detecting anomalies, and optimizing your cloud spend.
          </p>
          <a href="/settings"
            className="inline-block px-6 py-3 bg-blue-600 text-white rounded-lg font-medium hover:bg-blue-700 transition">
            Connect Cloud Provider
          </a>
        </div>
      </div>
    )
  }

  // Providers connected but no data yet
  if (hasProviders && !hasData) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-bold">Dashboard</h1>
        <div className="bg-white rounded-xl shadow-sm border border-gray-100 p-12 text-center">
          <div className="w-16 h-16 bg-yellow-100 rounded-full flex items-center justify-center mx-auto mb-4">
            <TrendingUp className="w-8 h-8 text-yellow-600" />
          </div>
          <h2 className="text-xl font-semibold mb-2">Syncing Your Data</h2>
          <p className="text-gray-500 mb-4 max-w-md mx-auto">
            Your cloud providers are connected. Cost data will appear here after the first sync completes.
            You can trigger a sync manually from Settings.
          </p>
          <div className="flex gap-2 justify-center">
            {providers?.map(p => (
              <span key={p.id || p.name} className={`px-3 py-1 rounded-full text-xs font-medium ${
                p.status === 'connected' ? 'bg-green-100 text-green-800' :
                p.status === 'error' ? 'bg-red-100 text-red-800' :
                'bg-yellow-100 text-yellow-800'
              }`}>
                {p.name} - {p.status}
              </span>
            ))}
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Dashboard</h1>
        <div className="flex gap-2">
          {providers?.map(p => (
            <span key={p.id || p.name} className={`px-3 py-1 rounded-full text-xs font-medium ${
              p.status === 'connected' ? 'bg-green-100 text-green-800' :
              p.status === 'error' ? 'bg-red-100 text-red-800' :
              'bg-yellow-100 text-yellow-800'
            }`}>
              {p.name}
            </span>
          ))}
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <StatCard title="Total Costs (30d)" value={`$${(summary?.total_cost || 0).toLocaleString()}`} icon={DollarSign} trend={trend?.change_percent} />
        <StatCard title="Active Anomalies" value={String(anomaliesData?.data?.filter(a => a.status === 'open').length || 0)} icon={AlertTriangle} />
        <StatCard title="Budgets" value={`${budgets?.data?.length || 0} active`} icon={TrendingUp} subtitle={`${budgets?.data?.filter(b => b.status === 'exceeded').length || 0} exceeded`} />
        <StatCard title="Potential Savings" value={`$${(recommendations?.data?.reduce((sum, r) => sum + r.estimated_savings, 0) || 0).toLocaleString()}`} icon={Lightbulb} />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="bg-white rounded-xl shadow-sm p-6 border border-gray-100">
          <h2 className="text-lg font-semibold mb-4">Cost Trend</h2>
          <ResponsiveContainer width="100%" height={300}>
            <LineChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="date" tick={{ fontSize: 12 }} />
              <YAxis tick={{ fontSize: 12 }} tickFormatter={(v) => `$${v}`} />
              <Tooltip formatter={(v: number) => [`$${v.toLocaleString()}`, 'Cost']} />
              <Line type="monotone" dataKey="cost" stroke="#3b82f6" strokeWidth={2} dot={false} />
            </LineChart>
          </ResponsiveContainer>
        </div>
        <div className="bg-white rounded-xl shadow-sm p-6 border border-gray-100">
          <h2 className="text-lg font-semibold mb-4">Cost by Service</h2>
          <ResponsiveContainer width="100%" height={300}>
            <PieChart>
              <Pie data={pieData} dataKey="value" nameKey="name" cx="50%" cy="50%" innerRadius={60} outerRadius={100} label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}>
                {pieData.map((_, i) => <Cell key={i} fill={COLORS[i % COLORS.length]} />)}
              </Pie>
              <Tooltip formatter={(v: number) => `$${v.toLocaleString()}`} />
            </PieChart>
          </ResponsiveContainer>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
          <div className="p-4 border-b border-gray-100"><h2 className="font-semibold">Recent Anomalies</h2></div>
          <div className="divide-y">
            {anomaliesData?.data?.slice(0, 5).map(a => (
              <div key={a.id} className="p-4 flex items-center justify-between">
                <div>
                  <p className="font-medium">{a.service || 'Unknown Service'}</p>
                  <p className="text-sm text-gray-500">{new Date(a.date).toLocaleDateString()}</p>
                </div>
                <div className="flex items-center gap-3">
                  <span className="text-sm text-red-600">{a.deviation_pct > 0 ? '+' : ''}{a.deviation_pct?.toFixed(1)}%</span>
                  <SeverityBadge severity={a.severity} />
                </div>
              </div>
            ))}
            {!anomaliesData?.data?.length && <p className="p-4 text-gray-500 text-center">No anomalies detected</p>}
          </div>
        </div>
        <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
          <div className="p-4 border-b border-gray-100"><h2 className="font-semibold">Top Recommendations</h2></div>
          <div className="divide-y">
            {recommendations?.data?.slice(0, 5).map(r => (
              <div key={r.id} className="p-4 flex items-center justify-between">
                <div>
                  <p className="font-medium">{r.resource_type}</p>
                  <p className="text-sm text-gray-500 capitalize">{r.type?.replace('_', ' ')}</p>
                </div>
                <span className="text-green-600 font-medium">${r.estimated_savings?.toLocaleString()}/mo</span>
              </div>
            ))}
            {!recommendations?.data?.length && <p className="p-4 text-gray-500 text-center">No recommendations</p>}
          </div>
        </div>
      </div>
    </div>
  )
}

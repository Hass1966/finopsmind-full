import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  DollarSign, TrendingUp, ArrowUpRight, LayoutDashboard, Search, Download
} from 'lucide-react'
import {
  XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
  PieChart, Pie, Cell, AreaChart, Area
} from 'recharts'
import { api } from '../lib/api'
import type { CostSummary, CostTrend } from '../lib/types'
import { StatCard, LoadingSpinner } from '../components/shared'

const COLORS = ['#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6', '#ec4899', '#14b8a6', '#f97316']
const API_URL = import.meta.env.VITE_API_URL || ''

export default function CostsPage() {
  const [serviceFilter, setServiceFilter] = useState('')
  const [dateRange, setDateRange] = useState('30')

  const { data: summary, isLoading: loadingSummary } = useQuery<CostSummary>({
    queryKey: ['costSummary'],
    queryFn: () => api.get('/costs/summary')
  })
  const { data: trend } = useQuery<CostTrend>({
    queryKey: ['costTrend', dateRange],
    queryFn: () => api.get(`/costs/trend?days=${dateRange}`)
  })

  const chartData = trend?.data_points?.map(d => ({
    date: new Date(d.date).toLocaleDateString('en-US', { month: 'short', day: 'numeric' }),
    cost: d.total
  })) || []

  const serviceData = summary?.by_service?.map((s, i) => ({
    name: s.name, amount: s.amount, percentage: s.percentage, color: COLORS[i % COLORS.length]
  })) || []

  const filteredServices = serviceFilter
    ? serviceData.filter(s => s.name.toLowerCase().includes(serviceFilter.toLowerCase()))
    : serviceData

  const handleExport = () => {
    const token = localStorage.getItem('finopsmind_token')
    const url = `${API_URL}/api/v1/costs/export?days=${dateRange}`
    const a = document.createElement('a')
    // Use fetch with auth header then trigger download
    fetch(url, { headers: token ? { Authorization: `Bearer ${token}` } : {} })
      .then(res => res.blob())
      .then(blob => {
        a.href = URL.createObjectURL(blob)
        a.download = `finopsmind-costs-${new Date().toISOString().split('T')[0]}.csv`
        a.click()
        URL.revokeObjectURL(a.href)
      })
  }

  if (loadingSummary) return <LoadingSpinner />

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Cost Analysis</h1>
          <p className="text-gray-500 text-sm mt-1">Detailed breakdown of your cloud spending</p>
        </div>
        <div className="flex gap-3">
          <select value={dateRange} onChange={(e) => setDateRange(e.target.value)}
            className="px-4 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500">
            <option value="7">Last 7 days</option>
            <option value="30">Last 30 days</option>
            <option value="90">Last 90 days</option>
          </select>
          <button onClick={handleExport} className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-700">
            <Download size={16} />
            Export
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        <StatCard title="Total Spend" value={`$${(summary?.total_cost || 0).toLocaleString()}`} icon={DollarSign} trend={summary?.change_percent} />
        <StatCard title="Daily Average" value={`$${((summary?.total_cost || 0) / parseInt(dateRange)).toFixed(2)}`} icon={TrendingUp} />
        <StatCard title="Top Service" value={serviceData[0]?.name || 'N/A'} subtitle={serviceData[0] ? `$${serviceData[0].amount.toLocaleString()}` : ''} icon={ArrowUpRight} />
        <StatCard title="Services Used" value={String(serviceData.length)} icon={LayoutDashboard} />
      </div>

      <div className="bg-white rounded-xl shadow-sm p-6 border border-gray-100">
        <h2 className="text-lg font-semibold mb-4">Daily Cost Trend</h2>
        <ResponsiveContainer width="100%" height={350}>
          <AreaChart data={chartData}>
            <defs>
              <linearGradient id="colorCost" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.3}/>
                <stop offset="95%" stopColor="#3b82f6" stopOpacity={0}/>
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey="date" tick={{ fontSize: 12 }} />
            <YAxis tick={{ fontSize: 12 }} tickFormatter={(v) => `$${v}`} />
            <Tooltip formatter={(v: number) => [`$${v.toLocaleString()}`, 'Cost']} />
            <Area type="monotone" dataKey="cost" stroke="#3b82f6" strokeWidth={2} fill="url(#colorCost)" />
          </AreaChart>
        </ResponsiveContainer>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="bg-white rounded-xl shadow-sm p-6 border border-gray-100">
          <h2 className="text-lg font-semibold mb-4">Cost Distribution</h2>
          <ResponsiveContainer width="100%" height={300}>
            <PieChart>
              <Pie data={serviceData.slice(0, 6)} dataKey="amount" nameKey="name" cx="50%" cy="50%" outerRadius={100}
                label={({ name, percent }) => `${name.split(' ')[1] || name} ${(percent * 100).toFixed(0)}%`}>
                {serviceData.slice(0, 6).map((entry, i) => <Cell key={i} fill={entry.color} />)}
              </Pie>
              <Tooltip formatter={(v: number) => `$${v.toLocaleString()}`} />
            </PieChart>
          </ResponsiveContainer>
        </div>

        <div className="bg-white rounded-xl shadow-sm p-6 border border-gray-100">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold">Service Breakdown</h2>
            <div className="relative">
              <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input type="text" placeholder="Filter services..." value={serviceFilter} onChange={(e) => setServiceFilter(e.target.value)}
                className="pl-9 pr-4 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
            </div>
          </div>
          <div className="space-y-3 max-h-64 overflow-y-auto">
            {filteredServices.map((service, i) => (
              <div key={i} className="flex items-center justify-between p-3 bg-gray-50 rounded-lg">
                <div className="flex items-center gap-3">
                  <div className="w-3 h-3 rounded-full" style={{ backgroundColor: service.color }} />
                  <span className="font-medium text-sm">{service.name}</span>
                </div>
                <div className="text-right">
                  <p className="font-semibold">${service.amount.toLocaleString()}</p>
                  <p className="text-xs text-gray-500">{service.percentage.toFixed(1)}%</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}

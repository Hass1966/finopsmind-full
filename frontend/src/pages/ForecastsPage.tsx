import { useQuery } from '@tanstack/react-query'
import {
  DollarSign, TrendingUp, CheckCircle, Clock
} from 'lucide-react'
import {
  Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
  AreaChart, Area, Legend
} from 'recharts'
import { api } from '../lib/api'
import type { CostTrend, Forecast } from '../lib/types'
import { StatCard, LoadingSpinner, EmptyState } from '../components/shared'

export default function ForecastsPage() {
  const { data: forecast, isLoading } = useQuery<{ data: Forecast[] }>({
    queryKey: ['forecasts'],
    queryFn: () => api.get('/forecasts')
  })
  const { data: trend } = useQuery<CostTrend>({
    queryKey: ['costTrend'],
    queryFn: () => api.get('/costs/trend')
  })

  const latestForecast = forecast?.data?.[0]

  const historicalData = trend?.data_points?.slice(-14).map(d => ({
    date: new Date(d.date).toLocaleDateString('en-US', { month: 'short', day: 'numeric' }),
    actual: d.total,
    predicted: null as number | null,
    lower: null as number | null,
    upper: null as number | null,
  })) || []

  const forecastData = latestForecast?.predictions?.map(p => ({
    date: new Date(p.date).toLocaleDateString('en-US', { month: 'short', day: 'numeric' }),
    actual: null as number | null,
    predicted: p.predicted,
    lower: p.lower_bound,
    upper: p.upper_bound,
  })) || []

  const combinedData = [...historicalData, ...forecastData]

  const totalForecast = latestForecast?.predictions?.reduce((sum, p) => sum + p.predicted, 0) || 0
  const avgDaily = totalForecast / (latestForecast?.predictions?.length || 1)
  const confidence = latestForecast?.confidence_level || 0

  if (isLoading) return <LoadingSpinner />

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Cost Forecasting</h1>
          <p className="text-gray-500 text-sm mt-1">ML-powered predictions for future cloud spending</p>
        </div>
        <div className="flex items-center gap-2 px-4 py-2 bg-blue-50 text-blue-700 rounded-lg">
          <TrendingUp size={16} />
          <span className="text-sm font-medium">Model: {latestForecast?.model_version || 'Prophet'}</span>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        <StatCard title="7-Day Forecast" value={`$${totalForecast.toLocaleString()}`} icon={TrendingUp} />
        <StatCard title="Avg Daily Predicted" value={`$${avgDaily.toFixed(2)}`} icon={DollarSign} />
        <StatCard title="Confidence Level" value={`${(confidence * 100).toFixed(0)}%`} icon={CheckCircle} />
        <StatCard title="Last Updated" value={latestForecast?.created_at ? new Date(latestForecast.created_at).toLocaleDateString() : 'N/A'} icon={Clock} />
      </div>

      <div className="bg-white rounded-xl shadow-sm p-6 border border-gray-100">
        <h2 className="text-lg font-semibold mb-4">Cost Forecast with Confidence Interval</h2>
        <ResponsiveContainer width="100%" height={400}>
          <AreaChart data={combinedData}>
            <defs>
              <linearGradient id="colorConfidence" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#10b981" stopOpacity={0.2}/>
                <stop offset="95%" stopColor="#10b981" stopOpacity={0}/>
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey="date" tick={{ fontSize: 12 }} />
            <YAxis tick={{ fontSize: 12 }} tickFormatter={(v) => `$${v}`} />
            <Tooltip formatter={(v) => v !== null ? `$${Number(v).toLocaleString()}` : 'N/A'} />
            <Legend />
            <Area type="monotone" dataKey="upper" stroke="transparent" fill="url(#colorConfidence)" name="Upper Bound" />
            <Area type="monotone" dataKey="lower" stroke="transparent" fill="#fff" name="Lower Bound" />
            <Line type="monotone" dataKey="actual" stroke="#3b82f6" strokeWidth={2} dot={{ r: 3 }} name="Actual Cost" connectNulls={false} />
            <Line type="monotone" dataKey="predicted" stroke="#10b981" strokeWidth={2} strokeDasharray="5 5" dot={{ r: 3 }} name="Predicted Cost" connectNulls={false} />
          </AreaChart>
        </ResponsiveContainer>
      </div>

      <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
        <div className="p-4 border-b border-gray-100"><h2 className="font-semibold">Daily Predictions</h2></div>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50 border-b border-gray-100">
              <tr>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Date</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Predicted Cost</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Lower Bound</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Upper Bound</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Range</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {latestForecast?.predictions?.map((p, i) => (
                <tr key={i} className="hover:bg-gray-50">
                  <td className="px-6 py-4 text-sm font-medium">{new Date(p.date).toLocaleDateString()}</td>
                  <td className="px-6 py-4 text-sm text-green-600 font-semibold">${p.predicted.toLocaleString()}</td>
                  <td className="px-6 py-4 text-sm text-gray-500">${p.lower_bound.toLocaleString()}</td>
                  <td className="px-6 py-4 text-sm text-gray-500">${p.upper_bound.toLocaleString()}</td>
                  <td className="px-6 py-4 text-sm text-gray-500">+/-${((p.upper_bound - p.lower_bound) / 2).toFixed(2)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {!latestForecast?.predictions?.length && (
          <EmptyState title="No forecasts available" description="Forecasts will appear once enough historical data is collected" icon={TrendingUp} />
        )}
      </div>
    </div>
  )
}

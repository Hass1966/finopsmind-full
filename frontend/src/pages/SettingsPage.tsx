import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../lib/api'
import type { Provider, OrgSettings } from '../lib/types'
import { LoadingSpinner } from '../components/shared'
import { useAuth } from '../contexts/AuthContext'

export default function SettingsPage() {
  const queryClient = useQueryClient()
  const { user, logout } = useAuth()

  const { data: providers, isLoading } = useQuery<Provider[]>({
    queryKey: ['providers'],
    queryFn: () => api.get('/providers')
  })

  const { data: settingsData } = useQuery<{ organization: string; settings: OrgSettings }>({
    queryKey: ['settings'],
    queryFn: () => api.get('/settings')
  })

  const [orgName, setOrgName] = useState('')
  const [settings, setSettings] = useState<OrgSettings>({
    default_currency: 'USD',
    timezone: 'UTC',
    fiscal_year_start: 1,
    alerts_enabled: true,
  })
  const [saveStatus, setSaveStatus] = useState<string | null>(null)

  useEffect(() => {
    if (settingsData) {
      setOrgName(settingsData.organization)
      setSettings(settingsData.settings)
    }
  }, [settingsData])

  const updateMutation = useMutation({
    mutationFn: (data: { name?: string; settings?: OrgSettings }) =>
      api.put('/settings', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      setSaveStatus('Settings saved successfully')
      setTimeout(() => setSaveStatus(null), 3000)
    },
    onError: () => {
      setSaveStatus('Failed to save settings')
      setTimeout(() => setSaveStatus(null), 3000)
    }
  })

  const handleSave = () => {
    updateMutation.mutate({ name: orgName, settings })
  }

  if (isLoading) return <LoadingSpinner />

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Settings</h1>
          <p className="text-gray-500 text-sm mt-1">Configure your FinOpsMind platform</p>
        </div>
        {user && (
          <div className="flex items-center gap-4">
            <span className="text-sm text-gray-500">{user.email}</span>
            <button onClick={logout} className="px-4 py-2 text-sm text-red-600 border border-red-200 rounded-lg hover:bg-red-50">
              Sign Out
            </button>
          </div>
        )}
      </div>

      {saveStatus && (
        <div className={`p-3 rounded-lg text-sm ${saveStatus.includes('success') ? 'bg-green-50 text-green-700 border border-green-200' : 'bg-red-50 text-red-700 border border-red-200'}`}>
          {saveStatus}
        </div>
      )}

      {/* Organization */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
        <div className="p-6 border-b border-gray-100">
          <h2 className="text-lg font-semibold">Organization</h2>
        </div>
        <div className="p-6 space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Organization Name</label>
            <input type="text" value={orgName} onChange={e => setOrgName(e.target.value)}
              className="w-full max-w-md px-4 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
          </div>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Default Currency</label>
              <select value={settings.default_currency} onChange={e => setSettings({ ...settings, default_currency: e.target.value })}
                className="w-full px-4 py-2 border border-gray-200 rounded-lg text-sm">
                <option value="USD">USD</option>
                <option value="EUR">EUR</option>
                <option value="GBP">GBP</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Timezone</label>
              <select value={settings.timezone} onChange={e => setSettings({ ...settings, timezone: e.target.value })}
                className="w-full px-4 py-2 border border-gray-200 rounded-lg text-sm">
                <option value="UTC">UTC</option>
                <option value="America/New_York">Eastern</option>
                <option value="America/Chicago">Central</option>
                <option value="America/Los_Angeles">Pacific</option>
                <option value="Europe/London">London</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Fiscal Year Start</label>
              <select value={settings.fiscal_year_start} onChange={e => setSettings({ ...settings, fiscal_year_start: parseInt(e.target.value) })}
                className="w-full px-4 py-2 border border-gray-200 rounded-lg text-sm">
                {Array.from({ length: 12 }, (_, i) => (
                  <option key={i + 1} value={i + 1}>{new Date(2000, i).toLocaleString('en-US', { month: 'long' })}</option>
                ))}
              </select>
            </div>
          </div>
        </div>
      </div>

      {/* Cloud Providers */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
        <div className="p-6 border-b border-gray-100">
          <h2 className="text-lg font-semibold">Cloud Providers</h2>
          <p className="text-sm text-gray-500 mt-1">Manage your connected cloud accounts</p>
        </div>
        <div className="divide-y divide-gray-100">
          {providers?.map(provider => (
            <div key={provider.name} className="p-6 flex items-center justify-between">
              <div className="flex items-center gap-4">
                <div className={`w-12 h-12 rounded-lg flex items-center justify-center ${
                  provider.name === 'aws' ? 'bg-orange-100' :
                  provider.name === 'gcp' ? 'bg-blue-100' :
                  provider.name === 'azure' ? 'bg-cyan-100' : 'bg-gray-100'
                }`}>
                  <span className="text-lg font-bold">
                    {provider.name === 'aws' ? 'AWS' :
                     provider.name === 'gcp' ? 'GCP' :
                     provider.name === 'azure' ? 'AZ' : provider.name.toUpperCase()}
                  </span>
                </div>
                <div>
                  <p className="font-medium capitalize">{provider.name}</p>
                  <p className="text-sm text-gray-500">{provider.type}</p>
                </div>
              </div>
              <span className={`px-3 py-1 rounded-full text-xs font-medium ${
                provider.healthy ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'
              }`}>
                {provider.healthy ? 'Connected' : 'Disconnected'}
              </span>
            </div>
          ))}
          {!providers?.length && (
            <div className="p-6 text-center text-gray-500">No providers configured</div>
          )}
        </div>
      </div>

      {/* Notifications */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
        <div className="p-6 border-b border-gray-100">
          <h2 className="text-lg font-semibold">Notifications</h2>
        </div>
        <div className="p-6">
          <div className="flex items-center justify-between py-3">
            <div>
              <p className="font-medium">Enable Alerts</p>
              <p className="text-sm text-gray-500">Receive anomaly, budget, and recommendation alerts</p>
            </div>
            <button onClick={() => setSettings({ ...settings, alerts_enabled: !settings.alerts_enabled })}
              className={`w-12 h-6 rounded-full transition-colors ${settings.alerts_enabled ? 'bg-blue-600' : 'bg-gray-200'}`}>
              <div className={`w-5 h-5 bg-white rounded-full shadow transition-transform ${settings.alerts_enabled ? 'translate-x-6' : 'translate-x-0.5'}`} />
            </button>
          </div>
        </div>
      </div>

      {/* Save Button */}
      <div className="flex justify-end">
        <button onClick={handleSave} disabled={updateMutation.isPending}
          className="px-6 py-3 bg-blue-600 text-white rounded-lg font-medium hover:bg-blue-700 disabled:opacity-50 transition">
          {updateMutation.isPending ? 'Saving...' : 'Save Settings'}
        </button>
      </div>

      {/* About */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
        <div className="p-6">
          <h2 className="text-lg font-semibold mb-4">About FinOpsMind</h2>
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div><p className="text-gray-500">Version</p><p className="font-medium">1.0.0</p></div>
            <div><p className="text-gray-500">ML Model</p><p className="font-medium">Prophet + Isolation Forest</p></div>
            <div><p className="text-gray-500">Backend</p><p className="font-medium">Go 1.23</p></div>
            <div><p className="text-gray-500">Frontend</p><p className="font-medium">React 18 + TypeScript</p></div>
          </div>
        </div>
      </div>
    </div>
  )
}

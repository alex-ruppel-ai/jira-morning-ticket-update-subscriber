import { useState, useEffect } from 'react'

interface Ticket {
  id: number
  jira_key: string
  summary: string
  added_at: string
}

interface Config {
  post_time: string
  timezone: string
  channel: string
}

const TIMEZONES = [
  { label: 'Pacific (PT)', value: 'America/Los_Angeles' },
  { label: 'Eastern (ET)', value: 'America/New_York' },
  { label: 'UTC', value: 'UTC' },
]

export default function DailyUpdateTab() {
  const [tickets, setTickets] = useState<Ticket[]>([])
  const [newKey, setNewKey] = useState('')
  const [addError, setAddError] = useState<string | null>(null)
  const [addLoading, setAddLoading] = useState(false)

  const [config, setConfig] = useState<Config>({ post_time: '09:00', timezone: 'America/Los_Angeles', channel: '' })
  const [configSaved, setConfigSaved] = useState(false)
  const [configError, setConfigError] = useState<string | null>(null)
  const [configLoading, setConfigLoading] = useState(false)

  const [triggerLoading, setTriggerLoading] = useState(false)
  const [triggerResult, setTriggerResult] = useState<string | null>(null)

  useEffect(() => {
    loadTickets()
    loadConfig()
  }, [])

  const loadTickets = async () => {
    try {
      const res = await fetch('/api/tickets')
      if (res.ok) {
        const data = await res.json()
        setTickets(data.tickets || [])
      }
    } catch {
      // ignore
    }
  }

  const loadConfig = async () => {
    try {
      const res = await fetch('/api/update-config')
      if (res.ok) {
        const data = await res.json()
        setConfig({
          post_time: data.post_time || '09:00',
          timezone: data.timezone || 'America/Los_Angeles',
          channel: data.channel || '',
        })
      }
    } catch {
      // ignore
    }
  }

  const handleAddTicket = async () => {
    const key = newKey.trim().toUpperCase()
    if (!key) return
    setAddLoading(true)
    setAddError(null)
    try {
      const res = await fetch('/api/tickets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ jira_key: key }),
      })
      if (!res.ok) {
        const data = await res.json()
        setAddError(data.error || 'Failed to add ticket')
        return
      }
      setNewKey('')
      await loadTickets()
    } catch {
      setAddError('Network error')
    } finally {
      setAddLoading(false)
    }
  }

  const handleRemoveTicket = async (key: string) => {
    try {
      await fetch(`/api/tickets/${key}`, { method: 'DELETE' })
      await loadTickets()
    } catch {
      // ignore
    }
  }

  const handleSaveConfig = async () => {
    setConfigLoading(true)
    setConfigError(null)
    setConfigSaved(false)
    try {
      const res = await fetch('/api/update-config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config),
      })
      if (!res.ok) {
        const data = await res.json()
        setConfigError(data.error || 'Failed to save')
        return
      }
      setConfigSaved(true)
      setTimeout(() => setConfigSaved(false), 2000)
    } catch {
      setConfigError('Network error')
    } finally {
      setConfigLoading(false)
    }
  }

  const handlePostNow = async () => {
    setTriggerLoading(true)
    setTriggerResult(null)
    try {
      const res = await fetch('/api/trigger-update', { method: 'POST' })
      if (!res.ok) {
        const data = await res.json()
        setTriggerResult('Error: ' + (data.error || 'Unknown error'))
        return
      }
      setTriggerResult('Posted! Check your Slack channel.')
    } catch {
      setTriggerResult('Network error')
    } finally {
      setTriggerLoading(false)
    }
  }

  return (
    <div className="max-w-2xl mx-auto space-y-8 py-4">

      {/* Tracked Tickets panel */}
      <div className="bg-white rounded-xl shadow p-6">
        <h2 className="text-xl font-semibold text-gray-800 mb-1">Tracked Tickets</h2>
        <p className="text-sm text-gray-500 mb-4">
          Add parent Jira ticket keys to track. The bot will follow all child tickets under each parent.
          Make sure Jira is connected in the integration bar above first.
        </p>

        <div className="flex gap-2 mb-3">
          <input
            type="text"
            value={newKey}
            onChange={e => setNewKey(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleAddTicket()}
            placeholder="e.g. NEU-12345"
            className="flex-1 border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
          <button
            onClick={handleAddTicket}
            disabled={addLoading || !newKey.trim()}
            className="bg-blue-600 hover:bg-blue-700 disabled:bg-gray-300 text-white px-4 py-2 rounded-lg text-sm font-medium"
          >
            {addLoading ? 'Adding...' : 'Add'}
          </button>
        </div>

        {addError && <p className="text-red-600 text-sm mb-3">{addError}</p>}

        {tickets.length === 0 ? (
          <p className="text-gray-400 text-sm">No tickets added yet.</p>
        ) : (
          <ul className="divide-y divide-gray-100">
            {tickets.map(t => (
              <li key={t.jira_key} className="flex items-center justify-between py-3">
                <div>
                  <span className="font-mono text-sm font-medium text-blue-700">{t.jira_key}</span>
                  <span className="text-gray-600 text-sm ml-2">{t.summary}</span>
                </div>
                <button
                  onClick={() => handleRemoveTicket(t.jira_key)}
                  className="text-gray-400 hover:text-red-600 text-sm ml-4"
                >
                  Remove
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>

      {/* Schedule Settings panel */}
      <div className="bg-white rounded-xl shadow p-6">
        <h2 className="text-xl font-semibold text-gray-800 mb-1">Schedule</h2>
        <p className="text-sm text-gray-500 mb-4">
          The bot will post a daily update to your Slack channel at the configured time.
          Use <strong>Post Now</strong> to send an update immediately.
        </p>

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Post time</label>
            <input
              type="time"
              value={config.post_time}
              onChange={e => setConfig(prev => ({ ...prev, post_time: e.target.value }))}
              className="border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Timezone</label>
            <select
              value={config.timezone}
              onChange={e => setConfig(prev => ({ ...prev, timezone: e.target.value }))}
              className="border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              {TIMEZONES.map(tz => (
                <option key={tz.value} value={tz.value}>{tz.label}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Slack channel</label>
            <input
              type="text"
              value={config.channel}
              onChange={e => setConfig(prev => ({ ...prev, channel: e.target.value }))}
              placeholder="#your-channel"
              className="border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 w-full"
            />
            <p className="text-xs text-gray-400 mt-1">Use #channel-name format. The bot must be invited to the channel.</p>
          </div>

          <div className="flex items-center gap-3 pt-2">
            <button
              onClick={handleSaveConfig}
              disabled={configLoading}
              className="bg-blue-600 hover:bg-blue-700 disabled:bg-gray-300 text-white px-5 py-2 rounded-lg text-sm font-medium"
            >
              {configLoading ? 'Saving...' : configSaved ? 'Saved!' : 'Save'}
            </button>
            <button
              onClick={handlePostNow}
              disabled={triggerLoading}
              className="bg-gray-700 hover:bg-gray-800 disabled:bg-gray-300 text-white px-5 py-2 rounded-lg text-sm font-medium"
            >
              {triggerLoading ? 'Posting...' : 'Post Now'}
            </button>
          </div>

          {configError && <p className="text-red-600 text-sm">{configError}</p>}
          {triggerResult && (
            <p className={`text-sm ${triggerResult.startsWith('Error') ? 'text-red-600' : 'text-green-600'}`}>
              {triggerResult}
            </p>
          )}
        </div>
      </div>
    </div>
  )
}

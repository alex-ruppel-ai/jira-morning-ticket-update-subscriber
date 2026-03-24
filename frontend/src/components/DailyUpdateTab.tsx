import { useState } from 'react'

export default function DailyUpdateTab() {
  const [jiraKey, setJiraKey] = useState('')
  const [channel, setChannel] = useState('')
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const handlePostNow = async () => {
    setLoading(true)
    setResult(null)
    setError(null)
    try {
      const res = await fetch('/api/trigger-update', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ jira_key: jiraKey.trim().toUpperCase(), channel: channel.trim() }),
      })
      const data = await res.json()
      if (!res.ok) {
        setError(data.error || 'Something went wrong')
        return
      }
      setResult('Posted! Check your Slack channel.')
    } catch {
      setError('Network error — could not reach the server.')
    } finally {
      setLoading(false)
    }
  }

  const canSubmit = jiraKey.trim() && channel.trim() && !loading

  return (
    <div className="max-w-lg mx-auto py-8 px-4">
      <div className="bg-white rounded-xl shadow p-6">
        <h2 className="text-xl font-semibold text-gray-800 mb-1">Daily Update</h2>
        <p className="text-sm text-gray-500 mb-6">
          Enter a Jira parent ticket and a Slack channel, then click Post Now to send an update.
          Make sure Jira is connected in the integration bar above.
        </p>

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Jira ticket key
            </label>
            <input
              type="text"
              value={jiraKey}
              onChange={e => setJiraKey(e.target.value)}
              placeholder="e.g. ADP-123"
              className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Slack channel
            </label>
            <input
              type="text"
              value={channel}
              onChange={e => setChannel(e.target.value)}
              placeholder="e.g. #adp-daily"
              className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <button
            onClick={handlePostNow}
            disabled={!canSubmit}
            className="w-full bg-blue-600 hover:bg-blue-700 disabled:bg-gray-300 disabled:cursor-not-allowed text-white font-medium py-2 px-4 rounded-lg text-sm transition-colors"
          >
            {loading ? 'Posting...' : 'Post Now'}
          </button>

          {result && (
            <p className="text-green-700 bg-green-50 border border-green-200 rounded-lg px-3 py-2 text-sm">
              {result}
            </p>
          )}
          {error && (
            <p className="text-red-700 bg-red-50 border border-red-200 rounded-lg px-3 py-2 text-sm">
              {error}
            </p>
          )}
        </div>
      </div>
    </div>
  )
}

function SlackSetupModal({ onClose }: { onClose: () => void }) {
  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-xl p-6 max-w-3xl w-full mx-4 max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-lg font-semibold text-gray-800">Connect to Slack</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600 text-xl leading-none">&times;</button>
        </div>

        <div className="space-y-6">
          <div>
            <h3 className="text-sm font-semibold text-gray-700 mb-2">1. Get Slack app approval from SamK</h3>
            <iframe
              src="https://drive.google.com/file/d/1qw7FnaK9dDSpjQpBFKqxtfYqK8Ycqgtf/preview"
              className="w-full rounded-lg border border-gray-200"
              height="360"
              allow="autoplay"
            />
          </div>

          <div>
            <h3 className="text-sm font-semibold text-gray-700 mb-2">2. Install and test your bot</h3>
            <iframe
              src="https://drive.google.com/file/d/1HP3c5xNWTwnv4ch95Gd5ZYYWFIw_NGqJ/preview"
              className="w-full rounded-lg border border-gray-200"
              height="360"
              allow="autoplay"
            />
          </div>
        </div>

        <p className="text-xs text-gray-500 mt-4"><strong>Note:</strong> Slack does not work when the app is run locally.</p>
        <button onClick={onClose} className="mt-4 w-full py-2 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-700">Got it</button>
      </div>
    </div>
  )
}

export default SlackSetupModal

/** Returns true if the response is a 503 "building" response from the supervisor,
 * indicating the Go backend is still compiling. */
export async function isBuildingResponse(res: Response): Promise<boolean> {
  if (res.status !== 503) return false
  try {
    const body = await res.clone().json()
    return body?.status === 'building'
  } catch {
    return false
  }
}


export function extractParamsFromDefinition(definition: string): string {
  if (!definition) return '{}'

  try {
    const params: Record<string, string> = {}
    // Regex to match $params.variableName
    const regex = /\$params\.([a-zA-Z0-9_]+)/g
    let match

    while ((match = regex.exec(definition)) !== null) {
      if (match[1]) {
        params[match[1]] = ''
      }
    }

    if (Object.keys(params).length === 0) {
      return '{}'
    }

    return JSON.stringify(params, null, 2)
  } catch (e) {
    console.error('Failed to extract params', e)
    return '{}'
  }
}

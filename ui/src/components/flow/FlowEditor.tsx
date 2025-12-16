import Editor from '@monaco-editor/react'
import { useTheme } from '../theme-provider'

interface FlowEditorProps {
  value: string
  onChange?: (value: string) => void
  language?: 'json' | 'yaml'
  readOnly?: boolean
}

export function FlowEditor({
  value,
  onChange,
  language = 'json',
  readOnly = false,
}: FlowEditorProps) {
  const { theme } = useTheme()

  const editorTheme = theme === 'dark' ? 'vs-dark' : 'light'

  return (
    <div className="border rounded-md overflow-hidden h-full flex flex-col">
      <div className="flex-1 min-h-[300px]">
        <Editor
          height="100%"
          defaultLanguage={language}
          language={language}
          value={value}
          theme={editorTheme}
          onChange={(val) => onChange?.(val || '')}
          options={{
            readOnly,
            minimap: { enabled: false },
            scrollBeyondLastLine: false,
            fontSize: 12,
            tabSize: 2,
            wordWrap: 'on',
          }}
        />
      </div>
    </div>
  )
}

export function highlightCode(src: string, symbols: string[] = []): string {
  let output = src.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  const store: string[] = []
  const stash = (className: string, text: string) => {
    store.push(`<span class="${className}">${text}</span>`)
    return `\x00${store.length - 1}\x00`
  }

  output = output.replace(/\/\/[^\n]*/g, (match) => stash('tok-com', match))
  output = output.replace(/&quot;.*?&quot;|"[^"]*"/g, (match) => stash('tok-str', match))
  output = output.replace(/\b(type|func|struct|string|uint|int|bool|return|const|var|interface|map|package|import)\b/g, '<span class="tok-key">$1</span>')

  const words = Array.from(new Set(symbols.filter(Boolean)))
  if (words.length > 0) {
    const pattern = new RegExp(`\\b(${words.map(escapeRegExp).join('|')})\\b`, 'g')
    output = output.replace(pattern, '<span class="tok-fn">$1</span>')
  }

  return output.replace(/\x00(\d+)\x00/g, (_, index) => store[Number(index)])
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

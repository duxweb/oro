#!/usr/bin/env node
const fs = require('fs')

const version = (process.argv[2] || process.env.GITHUB_REF_NAME || '').replace(/^v/, '')
const outputPath = process.argv[3]

if (!version) {
  console.error('missing version argument')
  process.exit(1)
}

function section(file, heading) {
  const lines = fs.readFileSync(file, 'utf8').replace(/\r\n/g, '\n').split('\n')
  const start = lines.findIndex((line) => line.startsWith(`## [${heading}]`))
  if (start < 0) {
    console.error(`missing changelog section ${heading} in ${file}`)
    process.exit(1)
  }

  let end = lines.length
  for (let index = start + 1; index < lines.length; index++) {
    if (lines[index].startsWith('## [')) {
      end = index
      break
    }
  }

  return lines.slice(start + 1, end).join('\n').trim()
}

const english = section('CHANGELOG.md', version)
const chinese = section('CHANGELOG.zh-CN.md', version)
const body = `# Oro v${version}\n\n## English\n\n${english}\n\n---\n\n## 简体中文\n\n${chinese}\n`

if (outputPath) {
  fs.writeFileSync(outputPath, body)
} else {
  process.stdout.write(body)
}

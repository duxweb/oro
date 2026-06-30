import type { APIRoute } from 'astro';
import { getCollection } from 'astro:content';

const base = 'https://duxweb.github.io/oro';

const docPath = (id: string) => id.replace(/(^|\/)index$/, '').replace(/\/$/, '');
const docURL = (id: string) => {
  const path = docPath(id);
  return path ? `${base}/${path}/` : `${base}/`;
};

export const GET: APIRoute = async () => {
  const docs = await getCollection('docs');
  const lines = [
    '# Oro',
    '',
    '> A humane, generic-first ORM for Go. No codegen, explicit schemas, typed generic queries, and a clean multi-driver architecture.',
    '',
    '## Docs',
    ...docs
      .filter((doc) => !doc.id.startsWith('zh-cn/'))
      .sort((a, b) => a.id.localeCompare(b.id))
      .map((doc) => {
        const description = doc.data.description ? `: ${doc.data.description}` : '';
        return `- [${doc.data.title}](${docURL(doc.id)})${description}`;
      }),
  ];

  return new Response(lines.join('\n'), {
    headers: { 'Content-Type': 'text/plain; charset=utf-8' },
  });
};

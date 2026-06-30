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
  const sections = docs
    .filter((doc) => !doc.id.startsWith('zh-cn/'))
    .sort((a, b) => a.id.localeCompare(b.id))
    .map((doc) => {
      const body = 'body' in doc && typeof doc.body === 'string' ? doc.body : '';
      return [
        `# ${doc.data.title}`,
        '',
        `URL: ${docURL(doc.id)}`,
        doc.data.description ? `Description: ${doc.data.description}` : '',
        '',
        body.trim(),
      ]
        .filter(Boolean)
        .join('\n');
    });

  const text = [
    '# Oro full documentation',
    '',
    '> A humane, generic-first ORM for Go.',
    '',
    ...sections,
  ].join('\n\n---\n\n');

  return new Response(text, {
    headers: { 'Content-Type': 'text/plain; charset=utf-8' },
  });
};

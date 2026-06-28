import { defineCollection } from 'astro:content';
import { glob } from 'astro/loaders';
import { velaFrontmatterSchema } from '@duxweb/vela/schema';
import { z } from 'astro/zod';

const homeSchema = velaFrontmatterSchema.extend({
  home: z
    .object({
      eyebrow: z.string().optional(),
      badge: z.string().optional(),
      install: z.string().optional(),
      codeTitle: z.string().optional(),
      code: z.string().optional(),
      pills: z.array(z.string()).optional(),
      symbols: z.array(z.string()).optional(),
    })
    .optional(),
});

export const collections = {
  docs: defineCollection({
    loader: glob({ base: './src/content/docs', pattern: '**/*.{md,mdx}' }),
    schema: homeSchema,
  }),
};

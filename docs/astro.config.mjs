import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  site: 'https://duxweb.github.io',
  vite: {
    plugins: [tailwindcss()],
  },
  integrations: [
    starlight({
      title: 'Oro',
      description: 'A humane, generic-first ORM for Go — no codegen, no import cycles, no black box.',
      favicon: '/favicon.svg',
      social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/duxweb/oro' }],
      components: {
        Hero: './src/components/Hero.astro',
      },
      customCss: ['./src/styles/tailwind.css', './src/styles/custom.css'],
      expressiveCode: {
        themes: ['github-dark'],
        styleOverrides: {
          borderRadius: '0.7rem',
          borderColor: 'transparent',
          frames: { shadowColor: 'transparent' },
        },
      },
      tableOfContents: { minHeadingLevel: 2, maxHeadingLevel: 3 },
      defaultLocale: 'root',
      locales: {
        root: {
          label: 'English',
          lang: 'en',
        },
        'zh-cn': {
          label: '简体中文',
          lang: 'zh-CN',
        },
      },
      sidebar: [
        {
          label: 'Start',
          translations: { 'zh-CN': '开始' },
          items: [
            { label: 'Overview', translations: { 'zh-CN': '概览' }, slug: '' },
            { label: 'Quick Start', translations: { 'zh-CN': '快速开始' }, slug: 'quick-start' },
            { label: 'Installation', translations: { 'zh-CN': '安装' }, slug: 'installation' }
          ]
        },
        {
          label: 'Guides',
          translations: { 'zh-CN': '指南' },
          items: [
            { label: 'Model Definition', translations: { 'zh-CN': '模型定义' }, slug: 'guides/model-definition' },
            { label: 'CRUD Queries', translations: { 'zh-CN': '增删查改' }, slug: 'guides/crud' },
            { label: 'Where Conditions', translations: { 'zh-CN': '查询条件' }, slug: 'guides/where' },
            { label: 'Relations', translations: { 'zh-CN': '关联关系' }, slug: 'guides/relations' },
            { label: 'Transactions', translations: { 'zh-CN': '事务' }, slug: 'guides/transactions' },
            { label: 'Schema Sync', translations: { 'zh-CN': '结构同步' }, slug: 'guides/schema-sync' },
            { label: 'Multiple Drivers', translations: { 'zh-CN': '多驱动' }, slug: 'guides/multiple-drivers' }
          ]
        },
        {
          label: 'Advanced',
          translations: { 'zh-CN': '进阶' },
          items: [
            { label: 'JSON & Full Text', translations: { 'zh-CN': 'JSON 与全文索引' }, slug: 'advanced/json-fulltext' },
            { label: 'Tenancy & Sharding', translations: { 'zh-CN': '租户与分片' }, slug: 'advanced/tenancy-sharding' },
            { label: 'Hooks & Events', translations: { 'zh-CN': 'Hooks 与事件' }, slug: 'advanced/hooks-events' },
            { label: 'Testing Matrix', translations: { 'zh-CN': '测试矩阵' }, slug: 'advanced/testing' }
          ]
        },
        {
          label: 'Reference',
          translations: { 'zh-CN': '参考' },
          items: [
            { label: 'API Naming', translations: { 'zh-CN': 'API 命名' }, slug: 'reference/api' },
            { label: 'Configuration', translations: { 'zh-CN': '配置' }, slug: 'reference/configuration' }
          ]
        }
      ]
    })
  ]
});

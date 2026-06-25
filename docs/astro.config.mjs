import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  site: 'https://duxweb.github.io',
  base: '/oro',
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
        ThemeSelect: './src/components/ThemeSelect.astro',
        LanguageSelect: './src/components/LanguageSelect.astro',
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
            { label: 'Installation', translations: { 'zh-CN': '安装' }, slug: 'installation' },
            { label: 'Concepts', translations: { 'zh-CN': '核心概念' }, slug: 'concepts' }
          ]
        },
        {
          label: 'Core Guides',
          translations: { 'zh-CN': '核心指南' },
          items: [
            { label: 'Model Definition', translations: { 'zh-CN': '模型定义' }, slug: 'guides/model-definition' },
            { label: 'Field Types', translations: { 'zh-CN': '字段类型' }, slug: 'guides/field-types' },
            { label: 'Schema Sync', translations: { 'zh-CN': '结构同步' }, slug: 'guides/schema-sync' },
            { label: 'Create', translations: { 'zh-CN': '创建数据' }, slug: 'guides/create' },
            { label: 'Query', translations: { 'zh-CN': '查询数据' }, slug: 'guides/query' },
            { label: 'Where Conditions', translations: { 'zh-CN': '查询条件' }, slug: 'guides/where' },
            { label: 'Select & Aggregate', translations: { 'zh-CN': '选择与聚合' }, slug: 'guides/select-aggregate-group' },
            { label: 'Update & Delete', translations: { 'zh-CN': '更新与删除' }, slug: 'guides/update-delete' },
            { label: 'Table & Raw', translations: { 'zh-CN': '裸表与原生 SQL' }, slug: 'guides/table-raw' },
            { label: 'Pagination', translations: { 'zh-CN': '分页与流式读取' }, slug: 'guides/pagination' },
            { label: 'Transactions', translations: { 'zh-CN': '事务' }, slug: 'guides/transactions' }
          ]
        },
        {
          label: 'Relations',
          translations: { 'zh-CN': '关联关系' },
          items: [
            { label: 'Definition', translations: { 'zh-CN': '关系定义' }, slug: 'relations/definition' },
            { label: 'Preloading', translations: { 'zh-CN': '预加载' }, slug: 'relations/preloading' },
            { label: 'Querying', translations: { 'zh-CN': '关联查询' }, slug: 'relations/querying' },
            { label: 'Aggregates', translations: { 'zh-CN': '关联聚合' }, slug: 'relations/aggregates' },
            { label: 'Writing', translations: { 'zh-CN': '关联写入' }, slug: 'relations/writing' },
            { label: 'Many-to-Many & Pivot', translations: { 'zh-CN': '多对多与中间表' }, slug: 'relations/many-to-many-pivot' }
          ]
        },
        {
          label: 'Advanced',
          translations: { 'zh-CN': '进阶' },
          items: [
            { label: 'Multi Driver & Connections', translations: { 'zh-CN': '多驱动与连接' }, slug: 'advanced/multiple-drivers' },
            { label: 'Driver Adapters', translations: { 'zh-CN': '驱动适配器' }, slug: 'advanced/driver-adapters' },
            { label: 'Hooks & Events', translations: { 'zh-CN': 'Hooks 与事件' }, slug: 'advanced/hooks-events' },
            { label: 'Scopes', translations: { 'zh-CN': '查询作用域' }, slug: 'advanced/scopes' },
            { label: 'Sharding', translations: { 'zh-CN': '分片' }, slug: 'advanced/sharding' },
            { label: 'JSON & Full Text', translations: { 'zh-CN': 'JSON 与全文索引' }, slug: 'advanced/json-fulltext' },
            { label: 'Cache', translations: { 'zh-CN': '查询缓存' }, slug: 'advanced/cache' },
            { label: 'Serialization', translations: { 'zh-CN': '序列化输出' }, slug: 'advanced/serialization' },
            { label: 'Error Handling', translations: { 'zh-CN': '错误处理' }, slug: 'advanced/error-handling' },
            { label: 'Logging', translations: { 'zh-CN': '日志' }, slug: 'advanced/logging' },
            { label: 'Testing Matrix', translations: { 'zh-CN': '测试矩阵' }, slug: 'advanced/testing' },
            { label: 'Performance Benchmarks', translations: { 'zh-CN': '性能基准' }, slug: 'advanced/performance-benchmarks' }
          ]
        },
        {
          label: 'Extensions',
          translations: { 'zh-CN': '扩展包' },
          items: [
            { label: 'Overview', translations: { 'zh-CN': '概览' }, slug: 'extensions' },
            { label: 'Tenant', translations: { 'zh-CN': 'Tenant 租户扩展' }, slug: 'extensions/tenant' },
            { label: 'Soft Delete', translations: { 'zh-CN': 'Soft Delete 软删除' }, slug: 'extensions/softdelete' },
            { label: 'Audit', translations: { 'zh-CN': 'Audit 审计' }, slug: 'extensions/audit' },
            { label: 'Metrics', translations: { 'zh-CN': 'Metrics 指标' }, slug: 'extensions/metrics' },
            { label: 'Nested Set', translations: { 'zh-CN': 'Nested Set 树形结构' }, slug: 'extensions/nestedset' }
          ]
        },
        {
          label: 'Reference',
          translations: { 'zh-CN': '参考' },
          items: [
            { label: 'API Naming', translations: { 'zh-CN': 'API 命名' }, slug: 'reference/api' },
            { label: 'Configuration', translations: { 'zh-CN': '配置' }, slug: 'reference/configuration' },
            { label: 'Field Builder', translations: { 'zh-CN': '字段构建器' }, slug: 'reference/field-builder' },
            { label: 'Query Builder', translations: { 'zh-CN': '查询构建器' }, slug: 'reference/query-builder' },
            { label: 'Conditions', translations: { 'zh-CN': '条件表达式' }, slug: 'reference/conditions' },
            { label: 'Write Options', translations: { 'zh-CN': '写入选项' }, slug: 'reference/write-options' },
            { label: 'Driver Interface', translations: { 'zh-CN': '驱动接口' }, slug: 'reference/driver-interface' },
            { label: 'Error Types', translations: { 'zh-CN': '错误类型' }, slug: 'reference/error-types' }
          ]
        }
      ]
    })
  ]
});

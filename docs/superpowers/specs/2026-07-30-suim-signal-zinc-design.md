# SuIM Signal Zinc 视觉设计

**日期：** 2026-07-30  
**状态：** 已批准（用户授权跳过后续确认，直接实现）

## Design Read

Reading this as: IM product shell redesign (auth + full chat UI) for everyday messaging users, with a Linear/Slack-clean language, leaning toward Tailwind utilities + zinc neutrals + single teal accent.

**Dials：** `DESIGN_VARIANCE: 4` / `MOTION_INTENSITY: 3` / `VISUAL_DENSITY: 6`

**模式：** Redesign - Overhaul visuals; preserve IA, routes, nav labels, and business logic.

## 决策摘要

| 项 | 选择 |
|---|---|
| 视觉方向 | A · Signal Zinc |
| 范围 | 全站：登录/注册 + 聊天壳 + 好友/群等面板 |
| 主题 | 浅色默认 + 完整深色（系统跟随 + 手动切换） |
| 实现路径 | Token 先行，原地重绘现有组件（不引入 shadcn） |

## 视觉 Token

### 颜色

**Light**
- `--surface`: `#fafafa`
- `--surface-elevated`: `#ffffff`
- `--surface-muted`: `#f4f4f5`
- `--ink`: `#18181b`
- `--ink-muted`: `#71717a`
- `--border`: `#e4e4e7`
- `--accent`: `#0d9488`
- `--accent-hover`: `#0f766e`
- `--accent-fg`: `#ffffff`
- `--rail`: `#18181b`

**Dark**
- `--surface`: `#09090b`
- `--surface-elevated`: `#18181b`
- `--surface-muted`: `#27272a`
- `--ink`: `#fafafa`
- `--ink-muted`: `#a1a1aa`
- `--border`: `#27272a`
- `--accent`: `#2dd4bf`
- `--accent-hover`: `#14b8a6`
- `--accent-fg`: `#09090b`
- `--rail`: `#09090b`

规则：全站唯一强调色 teal；废弃 indigo/purple 登录双轨与 unused clay 橙。

### 字体

- 西文/UI：Geist（`next/font`）或系统 UI 无衬线栈
- 中文：Noto Sans SC（`next/font`）
- 不使用 Inter 作为默认品牌字体声明（若系统回退到类似字体可接受）

### 形状

- 控件 / 输入 / 按钮：`8px`
- 消息气泡：`10px`（近发送端一侧可略尖 `4px`）
- 头像 / 品牌标：`8px` 方圆（非大 pill）
- 侧栏激活：细 teal 指示，无厚阴影卡片

### 动效

- 150–200ms ease；按钮 `active:scale-[0.98]`
- 面板切换短淡入
- 尊重 `prefers-reduced-motion`
- 本轮不引入 Motion/GSAP 编排

## 布局与组件

- 保持三栏 IA：SidebarNav | 列表面板 | ChatArea
- Auth：zinc 中性底 + elevated 表单卡 + teal CTA + SuIM 字标
- MessageBubble：己方 accent 实心；对方 elevated + border；系统 chip muted
- 好友/群/黑名单等：复用列表密度与边框语言
- 主题切换：个人资料或侧栏底部；`class="dark"` on `<html>`

## 本轮不做

- 改路由、导航标签、业务逻辑
- 引入 shadcn/Radix 大迁移
- 换图标库（保留 lucide，统一 stroke）
- 重做 Logo 矢量资产
- 后端 / SDK 改动

## 验收

- 登录与聊天视觉语言一致（无紫/emerald 分裂）
- 浅/深主题均可读，CTA 对比达标
- 主要交互表面（会话、消息、好友、群、资料）均使用 token
- 无 clay / indigo 主题残留

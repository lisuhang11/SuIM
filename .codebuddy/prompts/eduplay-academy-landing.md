# EduPlay Academy — 前端落地页开发提示词

> 本提示词供 `ui-ux-pro-max` Skill + `frontend-design` Skill + `ui-styling` Skill 联合迭代使用。
> 项目后端为 SuIM 微服务即时通讯系统，前端需对接其 REST API 与 WebSocket 长连接。

---

## 1. 项目背景与技术上下文

**后端能力概览（SuIM 微服务）**：
| 服务 | 端口 | 用途 |
|------|------|------|
| `apigateway` | 9000 (HTTP) | 统一 REST 入口，Gin 框架 |
| `msggateway` | 9001 (WS) | WebSocket 长连接，实时消息 |
| `user` | gRPC | 注册/登录/JWT/用户资料 |
| `group` | gRPC | 群组管理/成员/禁言 |
| `message` | gRPC | 消息收发/撤回/已读/删除 |
| `conversation` | gRPC | 会话管理/置顶/免打扰 |
| `push` | gRPC | 离线推送令牌管理 |

**前端技术栈建议**：Next.js 15 + React 19 + Tailwind CSS 4 + shadcn/ui + GSAP（动画）

**设计系统的构建方式**：
```bash
# 第一步：生成设计系统
python "$HOME/.codebuddy/skills/ui-ux-pro-max/scripts/search.py" \
  "playful educational platform online learning claymorphism vibrant engaging" \
  --design-system -p "EduPlay Academy" -f markdown \
  --variance 7 --motion 6 --density 3

# 第二步：持久化到项目
python "$HOME/.codebuddy/skills/ui-ux-pro-max/scripts/search.py" \
  "playful educational platform online learning claymorphism" \
  --design-system -p "EduPlay Academy" --persist \
  --output-dir "."

# 第三步：补充搜索
python "$HOME/.codebuddy/skills/ui-ux-pro-max/scripts/search.py" "claymorphism" --domain style
python "$HOME/.codebuddy/skills/ui-ux-pro-max/scripts/search.py" "education learning playful" --domain landing
python "$HOME/.codebuddy/skills/ui-ux-pro-max/scripts/search.py" "vibrant playful education" --domain color
```

---

## 2. 设计系统速查（来自 ui-ux-pro-max 搜索结果）

### 视觉风格：Claymorphism (General)
- **关键词**：Soft 3D, chunky, playful, toy-like, bubbly, thick borders (3-4px), double shadows, rounded (16-24px)
- **最佳场景**：Educational apps, children's apps, SaaS, creative tools, onboarding, casual games
- **光影**：Inner + outer double shadows (subtle, no hard lines)；软按压 (200ms ease-out)；fluffy 元素；smooth transitions
- **暗色模式**：Partial support ⚠，优先做 Light 版本
- **可访问性**：确保文字对比度 ≥4.5:1

### 配色方案
| 角色 | 色值 | CSS 变量 |
|------|------|----------|
| Primary | `#4F46E5` | `--color-primary` |
| On Primary | `#FFFFFF` | `--color-on-primary` |
| Secondary | `#818CF8` | `--color-secondary` |
| Accent / CTA | `#EA580C` | `--color-accent` |
| Background | `#EEF2FF` | `--color-background` |
| Foreground | `#1E1B4B` | `--color-foreground` |
| Muted | `#EBEEF8` | `--color-muted` |
| Border | `#C7D2FE` | `--color-border` |
| Destructive | `#DC2626` | `--color-destructive` |

### 字体方案
| 用途 | 字体 | Weight |
|------|------|--------|
| Headings | **Baloo 2** | 400 / 500 / 600 / 700 |
| Body | **Comic Neue** | 300 / 400 / 700 |
| Google Fonts Import | `@import url('https://fonts.googleapis.com/css2?family=Baloo+2:wght@400;500;600;700&family=Comic+Neue:wght@300;400;700&display=swap');` |

### 关键效果
- Inner + outer 双阴影（subtle, no hard lines）
- 软按压反馈（200ms ease-out scale 0.97）
- 圆角 16-24px（卡片/按钮）
- 厚边框 3-4px solid
- Fluffy 元素 + smooth transitions (cubic-bezier)

### 必须避免
- 枯燥/扁平的设计
- 缺少游戏化元素
- 没有进度视觉反馈

---

## 3. 页面结构 & 组件清单

### Section 1：Hero 区域（视频优先型）
- **布局**：全宽英雄区，深色 60% 遮罩叠加
- **内容**：
  - 标题：「让学习像游戏一样上瘾」/ "Learn Like You Play"
  - 副标题：强调互动式学习 + 实时陪伴
  - 主 CTA 按钮（Accent 色 `#EA580C`）：「立即加入 · 免费体验」
  - 次 CTA（outline）：「浏览课程」
- **Claymorphism 应用**：CTA 按钮使用双阴影（inner + outer），3px 粗边框，圆角 20px
- **微交互**：hover 时按钮弹跳（scale 1.05），点击 squish（scale 0.95）
- **动画**：背景有缓慢漂移的柔和色块（GSAP slow drift），文字逐行淡入

### Section 2：课程目录预览
- **布局**：3 列卡片网格（移动端单列），Bento Grid 风格
- **卡片设计**（Claymorphism）：
  - 圆角 20px，双阴影（inset -2px -2px 8px rgba(79,70,229,0.08), 6px 6px 16px rgba(79,70,229,0.12)）
  - 3-4px 实色边框（`#C7D2FE`）
  - 卡片顶部渐变条（Primary → Secondary 渐变）
  - 课程图标（SVG icon，用 Lucide）
  - 课程标题（Baloo 2, 600）
  - 课程描述（Comic Neue, 400）
  - 难度标签（pill badge，圆角 999px）
  - 学习时长 + 学生数量指标
- **状态**：每张卡片 hover 微浮起（translateY -4px + shadow 加深），过渡 250ms
- **数据**：静态 mock 展示 6 门课程（可扩展到 API 拉取）

### Section 3：进度追踪 Demo
- **布局**：左右分栏（移动端上下堆叠）
  - 左：统计卡片组（总学习时长、完成课程数、连续打卡天数）
  - 右：课程进度条列表 + 勋章墙
- **统计卡片**（Claymorphism）：
  - 大号数字（Baloo 2, 700, 48px）
  - 标签文字（Comic Neue, 400, muted 色）
  - 渐变图标背景圆
- **进度条**：
  - 圆角 999px，高度 12px
  - 渐变填充（Primary → Accent）
  - 百分比标签右侧显示
- **勋章墙**：
  - 已获得：完整色彩 + 双阴影
  - 未获得：灰色 + 单一虚线边框
- **动画**：数字递增滚动效果（count-up），进度条加载动画

### Section 4：学生感言
- **布局**：水平滚动轮播（3 列可见，移动端 1 列）
- **卡片设计**（Claymorphism）：
  - 白色卡片，圆角 20px，双阴影
  - 引用标记（大号 SVG quote icon，Primary 色 15% 透明度）
  - 评论文本（Comic Neue, 400, italic）
  - 星级评分（金色 `#F59E0B`）
  - 底部：头像（48px 圆形）+ 姓名 + 所学课程
- **交互**：自动轮播（5s 间隔），支持左右箭头手动切换，拖拽滑动
- **动画**：GSAP ScrollTrigger 触发，卡片从下淡入
- **数据**：5-8 条 mock 数据

### Section 5：报名 CTA
- **布局**：全宽背景色块（Secondary 渐变），圆角 24px
- **内容**：
  - 标题：「准备好开启你的学习冒险了吗？」
  - 描述：「10,000+ 学员已经加入，现在就是你最好的开始时机」
  - 邮箱输入框 + 提交按钮
- **输入框**（Claymorphism）：
  - 双阴影（inset 效果更明显）
  - 3px 边框（focus 时 Primary 色发光 ring）
  - 圆角 16px
- **按钮**：
  - Accent 色，大圆角，双阴影
  - hover: scale 1.05 + 阴影加深
  - click: scale 0.95（squish 动画）
- **动画**：背景有缓慢浮动的装饰圆球（GSAP）

### 全局组件
- **导航栏**：
  - Sticky 顶部，滚动后添加幕玻璃效果（backdrop-blur）
  - Logo + 导航链接 + Login / Sign Up 按钮
  - 移动端：汉堡菜单
- **页脚**：
  - 4 列网格（产品/资源/公司/法律）
  - 底部 Copyright + 社交媒体图标

---

## 4. Claymorphism 关键 CSS 技术参数

```css
/* 卡片 */
.clay-card {
  border-radius: 20px;
  border: 3px solid var(--color-border);
  background: var(--color-card, #FFFFFF);
  box-shadow:
    inset -2px -2px 8px rgba(79, 70, 229, 0.06),
    6px 6px 16px rgba(79, 70, 229, 0.10);
  transition: all 250ms cubic-bezier(0.34, 1.56, 0.64, 1);
}

.clay-card:hover {
  transform: translateY(-4px);
  box-shadow:
    inset -2px -2px 12px rgba(79, 70, 229, 0.08),
    8px 8px 24px rgba(79, 70, 229, 0.16);
}

/* 按钮 */
.clay-button {
  border-radius: 20px;
  border: 3px solid rgba(255, 255, 255, 0.3);
  padding: 14px 32px;
  font-family: 'Baloo 2', sans-serif;
  font-weight: 600;
  font-size: 18px;
  background: var(--color-accent);
  color: white;
  box-shadow:
    inset -2px -4px 8px rgba(0, 0, 0, 0.15),
    4px 6px 14px rgba(234, 88, 12, 0.30);
  transition: all 200ms ease-out;
  cursor: pointer;
}

.clay-button:hover {
  transform: scale(1.05);
  box-shadow:
    inset -2px -4px 8px rgba(0, 0, 0, 0.12),
    6px 8px 20px rgba(234, 88, 12, 0.40);
}

.clay-button:active {
  transform: scale(0.95);
  box-shadow:
    inset 2px 4px 8px rgba(0, 0, 0, 0.20),
    1px 2px 6px rgba(234, 88, 12, 0.15);
}

/* 输入框 */
.clay-input {
  border-radius: 16px;
  border: 3px solid var(--color-border);
  background: var(--color-card, #FFFFFF);
  padding: 14px 20px;
  font-family: 'Comic Neue', sans-serif;
  font-size: 16px;
  box-shadow:
    inset 2px 2px 6px rgba(79, 70, 229, 0.06),
    0 0 0 transparent;
  transition: all 200ms ease-out;
}

.clay-input:focus {
  outline: none;
  border-color: var(--color-primary);
  box-shadow:
    inset 2px 2px 6px rgba(79, 70, 229, 0.08),
    0 0 0 4px rgba(79, 70, 229, 0.15);
}

/* 进度条 */
.clay-progress {
  height: 12px;
  border-radius: 999px;
  background: var(--color-muted);
  box-shadow: inset 2px 2px 4px rgba(0,0,0,0.06);
  overflow: hidden;
}

.clay-progress-fill {
  height: 100%;
  border-radius: 999px;
  background: linear-gradient(90deg, var(--color-primary), var(--color-accent));
  box-shadow: 0 1px 3px rgba(79, 70, 229, 0.30);
  transition: width 800ms cubic-bezier(0.34, 1.56, 0.64, 1);
}
```

---

## 5. 前端架构建议

### 页面路由
```
/               → Landing Page（本提示词目标页面）
/dashboard      → 学员仪表盘（后续）
/courses        → 课程列表（后续）
/courses/[id]   → 课程详情 + 群聊（后续，对接 IM）
```

### 组件树
```
Layout
├── Navbar（sticky + glass effect）
├── main
│   ├── HeroSection
│   │   ├── FloatingBlobs（GSAP 漂移动画）
│   │   ├── HeroTitle
│   │   ├── HeroSubtitle
│   │   └── CTAButtons
│   ├── CourseCatalogSection
│   │   ├── SectionHeader
│   │   └── CourseCardGrid
│   │       └── CourseCard[] (clay-morphism)
│   │           ├── CourseIcon
│   │           ├── CourseTitle
│   │           ├── CourseDescription
│   │           └── CourseMeta（duration, students, level）
│   ├── ProgressTrackingSection
│   │   ├── SectionHeader
│   │   ├── StatCards
│   │   │   └── StatCard[]（count-up animation）
│   │   ├── ProgressBars
│   │   └── BadgeWall
│   │       └── BadgeItem[]（earned / locked）
│   ├── TestimonialsSection
│   │   ├── SectionHeader
│   │   └── TestimonialCarousel
│   │       └── TestimonialCard[]
│   │           ├── QuoteIcon
│   │           ├── ReviewText
│   │           ├── StarRating
│   │           └── AuthorInfo（avatar + name + course）
│   └── EnrollmentCTASection
│       ├── CTAHeading
│       ├── CTADescription
│       └── EmailForm
│           ├── ClayInput
│           └── ClayButton
└── Footer
    ├── FooterGrid（4 col）
    └── FooterBottom
```

### 数据流
- 静态文案：国际化友好，放入 `locales/zh-CN.json`
- Mock 数据：Next.js `generateStaticParams` + 静态 JSON
- 动画：GSAP `ScrollTrigger` + `useGSAP` hook
- 响应式：Tailwind `sm:` `md:` `lg:` `xl:` 断点

---

## 6. Tailwind 配置骨架

```js
// tailwind.config.js
export default {
  theme: {
    extend: {
      fontFamily: {
        display: ['Baloo 2', 'sans-serif'],
        body: ['Comic Neue', 'sans-serif'],
      },
      colors: {
        primary:   { DEFAULT: '#4F46E5', light: '#818CF8', dark: '#3730A3' },
        accent:    { DEFAULT: '#EA580C', light: '#F97316', dark: '#C2410C' },
        background:'#EEF2FF',
        foreground:'#1E1B4B',
        muted:     { DEFAULT: '#EBEEF8', fg: '#64748B' },
        border:    '#C7D2FE',
      },
      borderRadius: {
        clay: '20px',
      },
      boxShadow: {
        'clay-card':   'inset -2px -2px 8px rgba(79,70,229,0.06), 6px 6px 16px rgba(79,70,229,0.10)',
        'clay-card-hover': 'inset -2px -2px 12px rgba(79,70,229,0.08), 8px 8px 24px rgba(79,70,229,0.16)',
        'clay-button': 'inset -2px -4px 8px rgba(0,0,0,0.15), 4px 6px 14px rgba(234,88,12,0.30)',
        'clay-input':  'inset 2px 2px 6px rgba(79,70,229,0.06)',
      },
      animation: {
        'float':  'float 6s ease-in-out infinite',
        'squish': 'squish 200ms ease-out',
        'fade-up': 'fadeUp 600ms ease-out',
      },
      keyframes: {
        float: {
          '0%, 100%': { transform: 'translateY(0px)' },
          '50%':      { transform: 'translateY(-20px)' },
        },
        squish: {
          '0%':   { transform: 'scale(1)' },
          '50%':  { transform: 'scale(0.95)' },
          '100%': { transform: 'scale(1)' },
        },
        fadeUp: {
          '0%':   { opacity: '0', transform: 'translateY(24px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
      },
    },
  },
};
```

---

## 7. 迭代开发流程指引

### 第一轮：骨架 + 设计系统
```
请使用 ui-ux-pro-max 的 --design-system 输出作为设计依据，
用 Next.js + Tailwind CSS + shadcn/ui 搭建 EduPlay Academy 落地页的基础骨架：
1. 生成 design-system/eduplay-academy/MASTER.md
2. 创建 Layout、Navbar、Footer 组件
3. 创建 5 个 Section 的占位组件
4. 配置 Tailwind 主题（颜色、字体、圆角、阴影）
5. 确保 375px / 768px / 1024px / 1440px 四个断点基础响应
```

### 第二轮：Claymorphism 组件库
```
在当前骨架基础上，逐一实现 Claymorphism 风格的组件：
1. 实现 .clay-card 双阴影样式（参考本提示词 §4）
2. 实现 .clay-button 弹性按压效果
3. 实现 .clay-input focus 发光 ring
4. 实现 .clay-progress 渐变填充条
5. 确保所有组件在 hover / active / focus 三种状态视觉完整
6. 检查 prefers-reduced-motion 降级方案
```

### 第三轮：动画与微交互
```
为落地页添加动画：
1. Hero 区域浮动画色块（GSAP slow drift）
2. 课程卡片 hover 上浮 + shadow 加深
3. 进度条加载动画（从 0 到目标宽度，800ms ease-out）
4. 统计数字 count-up 递增效果
5. 感言轮播自动播放（5s）+ 拖拽滑动
6. CTA 按钮 squish 效果
7. ScrollTrigger 驱动的淡入动画
请参考 ui-ux-pro-max 中 --domain gsap 的动画预设
```

### 第四轮：内容填充 + 交付出厂
```
完整填充落地页内容：
1. Hero 文案（中英文双语）
2. 6 门课程 mock 数据（标题/描述/图标/时长/难度/学生数）
3. 5 条学生感言 mock 数据（姓名/头像/评分/评语/课程）
4. 页脚链接和社交媒体图标
5. 最终走查 pro-rules.md 的 Pre-Delivery Checklist
6. 验证暗色模式（Dark Mode Partial）
7. 验证可访问性：对比度 4.5:1，键盘导航，aria-labels
8. 性能检查：WebP 图片，lazy loading，CLS < 0.1
```

---

## 8. 可用 Skill 矩阵

| 阶段 | 使用的 Skill | 用途 |
|------|-------------|------|
| 设计系统生成 | `ui-ux-pro-max` | 搜索风格/配色/字体/UX规则 |
| 视觉设计方向 | `frontend-design` | 审美方向、排版、差异化决策 |
| 组件实现 | `ui-styling` | Tailwind + shadcn/ui 组件构建 |
| 动画实现 | `ui-ux-pro-max --domain gsap` | GSAP 动画预设 |
| 图表（后期） | `ui-ux-pro-max --domain chart` | 进度统计图表 |
| 品牌一致性 | `brand` | 品牌声音、视觉资产 |
| Banner 设计 | `banner-design` | 社交媒体投放素材 |

---

## 9. 快速启动命令摘要

```bash
# 1. 生成 + 持久化设计系统
python3 "$HOME/.codebuddy/skills/ui-ux-pro-max/scripts/search.py" \
  "playful educational platform online learning claymorphism vibrant engaging" \
  --design-system -p "EduPlay Academy" -f markdown \
  --variance 7 --motion 6 --density 3 \
  --persist --output-dir "."

# 2. 风格深入
python3 "$HOME/.codebuddy/skills/ui-ux-pro-max/scripts/search.py" "claymorphism" --domain style
python3 "$HOME/.codebuddy/skills/ui-ux-pro-max/scripts/search.py" "scroll reveal stagger card" --domain gsap

# 3. UX 验证
python3 "$HOME/.codebuddy/skills/ui-ux-pro-max/scripts/search.py" "animation accessibility loading" --domain ux

# 4. 创建前端项目
npx create-next-app@latest eduplay-academy --typescript --tailwind --eslint --app --src-dir
```

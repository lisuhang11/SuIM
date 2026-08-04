// ============================================================
// 工具函数
// ============================================================

/** 合并 class names */
export function cn(...classes: (string | undefined | null | false)[]): string {
  return classes.filter(Boolean).join(" ");
}

/** 将秒/毫秒时间戳统一为毫秒 */
export function toEpochMs(value: number | string): number {
  const n = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(n) || n <= 0) return NaN;
  // < 1e12 视为秒（当前秒级约 1.7e9，毫秒级约 1.7e12）
  return n < 1e12 ? n * 1000 : n;
}

function startOfLocalDay(d: Date): number {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
}

/** 24 小时制 HH:mm（不用 locale，避免 h11 把 12 点显示成 00:00） */
function formatClock24(d: Date): string {
  const h = String(d.getHours()).padStart(2, "0");
  const m = String(d.getMinutes()).padStart(2, "0");
  return `${h}:${m}`;
}

/** 格式化时间：今天 HH:mm；昨天 HH:mm；更早 YYYY-MM-DD HH:mm */
export function formatTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const now = new Date();
  const dayDiff = Math.round((startOfLocalDay(now) - startOfLocalDay(d)) / 86400000);
  const time = formatClock24(d);

  if (dayDiff === 0) return time;
  if (dayDiff === 1) return `昨天 ${time}`;

  const y = d.getFullYear();
  const mo = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${mo}-${day} ${time}`;
}

/** 格式化会话列表时间（简短版） */
export function formatConvTime(iso: string): string {
  const d = new Date(iso);
  const now = new Date();
  const diff = now.getTime() - d.getTime();
  const mins = Math.floor(diff / 60000);
  const hours = Math.floor(diff / 3600000);
  const days = Math.floor(diff / 86400000);

  if (mins < 1) return "刚刚";
  if (mins < 60) return `${mins}分钟前`;
  if (hours < 24) return `${hours}小时前`;
  if (days < 2) return "昨天";
  if (days < 7) return `${days}天前`;
  return d.toLocaleDateString("zh-CN", { month: "short", day: "numeric" });
}

/** 截断文本 */
export function truncate(text: string, maxLen: number): string {
  if (text.length <= maxLen) return text;
  return text.slice(0, maxLen) + "...";
}

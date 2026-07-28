// ============================================================
// 用户在线状态工具函数
// ============================================================
import type { UserStatus } from "@/types";

export function getStatusColor(status: UserStatus): string {
  switch (status) {
    case "online":  return "#22c55e";
    case "away":    return "#f59e0b";
    case "busy":    return "#ef4444";
    case "offline": return "#9ca3af";
  }
}

export function getStatusText(status: UserStatus): string {
  switch (status) {
    case "online":  return "在线";
    case "away":    return "离开";
    case "busy":    return "忙碌";
    case "offline": return "离线";
  }
}

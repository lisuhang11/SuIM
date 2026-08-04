// ============================================================
// ThemeToggle — 浅色 / 深色 / 跟随系统
// ============================================================
"use client";

import { Monitor, Moon, Sun } from "lucide-react";
import { useTheme, type ThemeMode } from "@/contexts/ThemeContext";

const OPTIONS: { mode: ThemeMode; label: string; icon: typeof Sun }[] = [
  { mode: "light", label: "浅色", icon: Sun },
  { mode: "dark", label: "深色", icon: Moon },
  { mode: "system", label: "系统", icon: Monitor },
];

export function ThemeToggle({ compact = false }: { compact?: boolean }) {
  const { mode, setMode } = useTheme();

  return (
    <div
      className={
        compact
          ? "inline-flex rounded-control border border-edge bg-surface-elevated p-0.5"
          : "flex w-full rounded-control border border-edge bg-surface-elevated p-1"
      }
      role="group"
      aria-label="主题"
    >
      {OPTIONS.map(({ mode: value, label, icon: Icon }) => {
        const active = mode === value;
        return (
          <button
            key={value}
            type="button"
            title={label}
            aria-pressed={active}
            onClick={() => setMode(value)}
            className={
              compact
                ? `inline-flex h-7 w-7 items-center justify-center rounded-[6px] transition duration-150 ${
                    active
                      ? "bg-accent text-accent-fg"
                      : "text-ink-muted hover:bg-surface-muted hover:text-ink"
                  }`
                : `flex flex-1 items-center justify-center gap-1.5 rounded-[6px] px-2 py-1.5 text-xs font-medium transition duration-150 ${
                    active
                      ? "bg-accent text-accent-fg"
                      : "text-ink-muted hover:bg-surface-muted hover:text-ink"
                  }`
            }
          >
            <Icon className="h-3.5 w-3.5" strokeWidth={1.75} />
            {!compact && <span>{label}</span>}
          </button>
        );
      })}
    </div>
  );
}

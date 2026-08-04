import type { Config } from "tailwindcss";

export default {
  darkMode: "class",
  content: ["./src/**/*.{js,ts,jsx,tsx,mdx}"],
  theme: {
    extend: {
      fontFamily: {
        sans: ["var(--font-sans)", "var(--font-cn)", "system-ui", "sans-serif"],
        cn: ["var(--font-cn)", "var(--font-sans)", "system-ui", "sans-serif"],
      },
      colors: {
        surface: {
          DEFAULT: "var(--surface)",
          elevated: "var(--surface-elevated)",
          muted: "var(--surface-muted)",
        },
        ink: {
          DEFAULT: "var(--ink)",
          muted: "var(--ink-muted)",
        },
        edge: "var(--border)",
        accent: {
          DEFAULT: "var(--accent)",
          hover: "var(--accent-hover)",
          fg: "var(--accent-fg)",
          soft: "var(--accent-soft)",
        },
        rail: "var(--rail)",
        danger: {
          DEFAULT: "var(--danger)",
          soft: "var(--danger-soft)",
        },
      },
      borderRadius: {
        control: "8px",
        bubble: "10px",
      },
      boxShadow: {
        panel: "0 12px 40px var(--shadow-panel)",
      },
      transitionDuration: {
        ui: "160ms",
      },
    },
  },
  plugins: [],
} satisfies Config;

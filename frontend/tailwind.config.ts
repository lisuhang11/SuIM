import type { Config } from "tailwindcss";

export default {
  content: ["./src/**/*.{js,ts,jsx,tsx,mdx}"],
  theme: {
    extend: {
      fontFamily: {
        display: ["Baloo 2", "sans-serif"],
        body: ["Comic Neue", "sans-serif"],
      },
      colors: {
        primary: {
          DEFAULT: "#4F46E5",
          light: "#818CF8",
          dark: "#3730A3",
        },
        accent: {
          DEFAULT: "#EA580C",
          light: "#F97316",
          dark: "#C2410C",
        },
        background: "#EEF2FF",
        foreground: "#1E1B4B",
        muted: {
          DEFAULT: "#EBEEF8",
          fg: "#64748B",
        },
        border: "#C7D2FE",
      },
      borderRadius: {
        clay: "20px",
      },
      boxShadow: {
        "clay-card":
          "inset -2px -2px 8px rgba(79,70,229,0.06), 6px 6px 16px rgba(79,70,229,0.10)",
        "clay-card-hover":
          "inset -2px -2px 12px rgba(79,70,229,0.08), 8px 8px 24px rgba(79,70,229,0.16)",
        "clay-button":
          "inset -2px -4px 8px rgba(0,0,0,0.15), 4px 6px 14px rgba(234,88,12,0.30)",
        "clay-button-hover":
          "inset -2px -4px 8px rgba(0,0,0,0.12), 6px 8px 20px rgba(234,88,12,0.40)",
        "clay-button-active":
          "inset 2px 4px 8px rgba(0,0,0,0.20), 1px 2px 6px rgba(234,88,12,0.15)",
        "clay-input": "inset 2px 2px 6px rgba(79,70,229,0.06)",
        "clay-badge":
          "inset -1px -1px 3px rgba(79,70,229,0.06), 2px 2px 6px rgba(79,70,229,0.10)",
        "clay-stat":
          "inset -2px -2px 6px rgba(79,70,229,0.05), 4px 4px 12px rgba(79,70,229,0.08)",
      },
      animation: {
        float: "float 6s ease-in-out infinite",
        "float-delayed": "float 8s ease-in-out 2s infinite",
        "float-slow": "float 10s ease-in-out 4s infinite",
        squish: "squish 200ms ease-out",
        "fade-up": "fadeUp 600ms ease-out forwards",
        "fade-in": "fadeIn 500ms ease-out forwards",
        "slide-in-left": "slideInLeft 600ms ease-out forwards",
        "slide-in-right": "slideInRight 600ms ease-out forwards",
        "count-up": "countUp 2s ease-out forwards",
        "progress-fill": "progressFill 1s ease-out forwards",
        "badge-pop": "badgePop 500ms cubic-bezier(0.34,1.56,0.64,1) forwards",
      },
      keyframes: {
        float: {
          "0%, 100%": { transform: "translateY(0px) rotate(0deg)" },
          "50%": { transform: "translateY(-20px) rotate(3deg)" },
        },
        squish: {
          "0%": { transform: "scale(1)" },
          "50%": { transform: "scale(0.95)" },
          "100%": { transform: "scale(1)" },
        },
        fadeUp: {
          "0%": { opacity: "0", transform: "translateY(24px)" },
          "100%": { opacity: "1", transform: "translateY(0)" },
        },
        fadeIn: {
          "0%": { opacity: "0" },
          "100%": { opacity: "1" },
        },
        slideInLeft: {
          "0%": { opacity: "0", transform: "translateX(-40px)" },
          "100%": { opacity: "1", transform: "translateX(0)" },
        },
        slideInRight: {
          "0%": { opacity: "0", transform: "translateX(40px)" },
          "100%": { opacity: "1", transform: "translateX(0)" },
        },
        progressFill: {
          "0%": { width: "0%" },
        },
        badgePop: {
          "0%": { opacity: "0", transform: "scale(0.5)" },
          "100%": { opacity: "1", transform: "scale(1)" },
        },
      },
      transitionTimingFunction: {
        bounce: "cubic-bezier(0.34, 1.56, 0.64, 1)",
      },
    },
  },
  plugins: [],
} satisfies Config;

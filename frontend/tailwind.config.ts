import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./src/pages/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/components/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/app/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        // Brand colors from .branding/logo.svg
        primary: "#009e20",
        accent: "#00a020",
        // Map slate to GitHub Dark Palette
        slate: {
          50: "#f0f6fc",
          100: "#c9d1d9",
          200: "#b1bac4",
          300: "#8b949e",
          400: "#6e7681",
          500: "#484f58",
          600: "#30363d",
          700: "#21262d",
          800: "#161b22",
          900: "#0d1117",
          950: "#010409",
        },
        // Legacy dark object (can be redundant but safe to keep)
        dark: {
          DEFAULT: "#0d1117",
          secondary: "#161b22",
          tertiary: "#21262d",
        },
        success: "#009e20",
        warning: "#d29922",
        error: "#f85149",
      },
      fontWeight: {
        thin: "200",
        extralight: "300",
        light: "400",
        normal: "500", // Scaled 1.15x (was 400)
        medium: "600", // Scaled (was 500)
        semibold: "700", // Scaled (was 600)
        bold: "800", // Scaled (was 700)
        extrabold: "900", // Scaled (was 800)
        black: "950",
      },
      fontFamily: {
        mono: ["Terminus", "Consolas", "Monaco", "monospace"],
      },
    },
  },
  plugins: [],
};
export default config;

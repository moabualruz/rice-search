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
        // No neon colors - using refined palette
        dark: {
          DEFAULT: "#0d1117",
          secondary: "#161b22",
          tertiary: "#21262d",
        },
        success: "#009e20",
        warning: "#d29922",
        error: "#f85149",
      },
      fontFamily: {
        mono: ["Terminus", "Consolas", "Monaco", "monospace"],
      },
    },
  },
  plugins: [],
};
export default config;

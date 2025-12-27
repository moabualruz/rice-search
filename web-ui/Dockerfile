# Rice Search - Web UI (Next.js with Bun)
# Multi-stage build for optimized production image

# ============================================
# Stage 1: Install dependencies with Bun
# ============================================
FROM oven/bun:1-alpine AS deps

WORKDIR /app

# Copy package files
COPY package.json bun.lockb* ./

# Install dependencies
RUN bun install --frozen-lockfile || bun install

# ============================================
# Stage 2: Build with Bun
# ============================================
FROM oven/bun:1-alpine AS builder

WORKDIR /app

# Copy node_modules from deps stage
COPY --from=deps /app/node_modules ./node_modules

# Copy application files
COPY . .

ENV NEXT_TELEMETRY_DISABLED 1

# Build the application
RUN bun run build

# ============================================
# Stage 3: Production image
# ============================================
FROM oven/bun:1-alpine AS runner

WORKDIR /app

ENV NODE_ENV production
ENV NEXT_TELEMETRY_DISABLED 1

# Create user and group for security
RUN addgroup --system --gid 1001 nodejs && \
    adduser --system --uid 1001 nextjs

# Copy public assets
COPY --from=builder /app/public ./public

# Set the correct permission for prerender cache
RUN mkdir .next && chown nextjs:nodejs .next

# Copy standalone build output
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static

USER nextjs

EXPOSE 3000

ENV PORT 3000
ENV HOSTNAME "0.0.0.0"

# Run with Bun's Node.js compatibility
CMD ["bun", "run", "server.js"]

FROM node:22-alpine

WORKDIR /app

# Copy both package files for proper npm install
COPY package.json package-lock.json* ./
COPY . .
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Next.js telemetry disable
ENV NEXT_TELEMETRY_DISABLED 1

ENTRYPOINT ["/entrypoint.sh"]
CMD ["npm", "run", "dev"]

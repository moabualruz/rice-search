FROM node:22-alpine

WORKDIR /app

# Copy both package files for proper npm install
COPY package.json package-lock.json* ./
RUN npm install

COPY . .

# Next.js telemetry disable
ENV NEXT_TELEMETRY_DISABLED 1

CMD ["npm", "run", "dev"]

FROM node:22-alpine

WORKDIR /app

# Don't copy source code - it will be mounted as a volume!
# Only copy the entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Next.js telemetry disable
ENV NEXT_TELEMETRY_DISABLED 1

ENTRYPOINT ["/entrypoint.sh"]
CMD ["npm", "run", "dev"]

/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        destination: process.env.API_URL || 'http://unified-api:8080/:path*',
      },
    ];
  },
};

module.exports = nextConfig;

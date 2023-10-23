/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  env: {
    NEXT_PROVIDER_HOST: process.env.NEXT_PROVIDER_HOST
  }
}

module.exports = nextConfig

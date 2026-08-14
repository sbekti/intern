const publicHostname = process.env.AUTH_PUBLIC_BASE_URL
  ? new URL(process.env.AUTH_PUBLIC_BASE_URL).hostname
  : undefined

/** @type {import('next').NextConfig} */
const nextConfig = {
  output: "standalone",
  allowedDevOrigins: publicHostname ? [publicHostname] : [],
}

export default nextConfig

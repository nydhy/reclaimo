const apiUrl = process.env.RECLAIMO_API_URL ?? "http://127.0.0.1:8081";

/** @type {import('next').NextConfig} */
const nextConfig = {
  async rewrites() {
    return [
      {
        source: "/lapdog/:path*",
        destination: "http://127.0.0.1:8126/:path*",
      },
      {
        source: "/reclaimo-api/:path*",
        destination: `${apiUrl}/:path*`,
      },
    ];
  },
};

export default nextConfig;

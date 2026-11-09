import type { NextConfig } from 'next';
const nextConfig: NextConfig = {
 reactStrictMode: true,
 poweredByHeader: false,
 async rewrites() {
  return [{source:'/api/:path*',destination:`${process.env.API_INTERNAL_URL||'http://127.0.0.1:8080'}/api/:path*`}];
 },
 async headers() {
  return [{source:'/(.*)',headers:[
   {key:'X-Content-Type-Options',value:'nosniff'},
   {key:'Referrer-Policy',value:'strict-origin-when-cross-origin'},
   {key:'X-Frame-Options',value:'DENY'},
   {key:'Permissions-Policy',value:'camera=(), microphone=(), geolocation=()'},
  ]}];
 }
};
export default nextConfig;

import type { NextConfig } from 'next';
import { initOpenNextCloudflareForDev } from '@opennextjs/cloudflare';

// Lets `next dev` see Cloudflare bindings; a no-op in production builds.
initOpenNextCloudflareForDev();

const nextConfig: NextConfig = {};

export default nextConfig;

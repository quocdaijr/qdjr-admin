import { defineCloudflareConfig } from '@opennextjs/cloudflare';

// No incremental cache: every admin page is client-rendered behind auth.
export default defineCloudflareConfig();

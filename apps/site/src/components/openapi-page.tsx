'use client';

// createOpenAPIPage() is a client-only factory — calling it directly inside
// mdx.tsx (imported by the server-rendered [[...slug]]/page.tsx) fails build
// with "Attempted to call createOpenAPIPage() from the server". Isolated
// here so mdx.tsx only ever references the already-built component, never
// invokes the factory itself during server-side module evaluation.
import { createOpenAPIPage } from 'fumadocs-openapi/ui';

export const OpenAPIPage = createOpenAPIPage();

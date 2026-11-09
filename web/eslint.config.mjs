import {dirname} from 'node:path';
import {fileURLToPath} from 'node:url';
import {FlatCompat} from '@eslint/eslintrc';
const compat=new FlatCompat({baseDirectory:dirname(fileURLToPath(import.meta.url))});
// Next.js generates next-env.d.ts; TypeScript validates it, but ESLint skips framework-owned references.
const config=[...compat.extends('next/core-web-vitals','next/typescript'),{ignores:['next-env.d.ts','.next/**','out/**']}];
export default config;

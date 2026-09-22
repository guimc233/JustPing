import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/**
 * Renders a version for display with exactly one leading "v".
 *
 * Release tags are stored and transmitted verbatim ("v1.2.1") but consumers may
 * also hold a bare version ("1.2.1"), so prepending "v" blindly yields "vv1.2.1".
 * Placeholders such as "dev" or "unknown" are passed through untouched rather
 * than decorated into "vdev".
 */
export function formatVersion(value: string | null | undefined, fallback = 'unknown'): string {
  const raw = (value ?? '').trim()
  if (raw === '') return fallback
  const clean = raw.replace(/^[vV]/, '')
  return /^\d/.test(clean) ? `v${clean}` : raw
}

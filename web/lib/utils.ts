import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function hasForcedGlimmer(
  params: Record<string, string | string[] | undefined>
) {
  const raw = params.glimmer
  const value = Array.isArray(raw) ? raw[0] : raw
  return value === "1"
}

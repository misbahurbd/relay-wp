const VARIABLE = /\{\{(\d+)\}\}/g

/** Replaces {{n}} with the n-th sample, or "[sample n]" while it's empty. */
export function fillVariables(text: string, samples: string[]) {
  return text.replace(VARIABLE, (_, n: string) => samples[Number(n) - 1]?.trim() || `[sample ${n}]`)
}

// The ten NCAA DI weight classes. Also the route-param whitelist: any /[weight]
// or /api/rankings/[weight] value outside this list is a 404.
export const WEIGHT_CLASSES = [125, 133, 141, 149, 157, 165, 174, 184, 197, 285] as const

export type WeightClass = (typeof WEIGHT_CLASSES)[number]

export function isWeightClass(value: number): value is WeightClass {
  return (WEIGHT_CLASSES as readonly number[]).includes(value)
}

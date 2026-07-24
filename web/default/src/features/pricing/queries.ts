import { queryOptions } from '@tanstack/react-query'
import { getPricing } from './api'

export const pricingQueryOptions = queryOptions({
  queryKey: ['pricing'],
  queryFn: getPricing,
  staleTime: 5 * 60 * 1000,
})

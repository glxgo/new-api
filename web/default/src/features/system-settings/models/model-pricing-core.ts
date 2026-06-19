/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import * as z from 'zod'
import {
  BILLING_VAR_REGEX,
  combineBillingExpr,
  splitBillingExprAndRequestRules,
} from '@/features/pricing/lib/billing-expr'
import { formatPricingNumber } from './pricing-format'

export const createModelPricingSchema = (t: (key: string) => string) =>
  z.object({
    name: z.string().min(1, t('Model name is required')),
    price: z.string().optional(),
    ratio: z.string().optional(),
    cacheRatio: z.string().optional(),
    createCacheRatio: z.string().optional(),
    completionRatio: z.string().optional(),
    imageRatio: z.string().optional(),
    audioRatio: z.string().optional(),
    audioCompletionRatio: z.string().optional(),
    // 成本价（平台买入价 $/1M tokens），与售价完全分离，仅超管可写。
    costInput: z.string().optional(),
    costOutput: z.string().optional(),
    costCache: z.string().optional(),
    // 倍率定价模式（multiplier）：官方价 + 销售/成本倍率，提交时换算成上面的 ratio/cost。
    officialInput: z.string().optional(),
    officialOutput: z.string().optional(),
    officialCacheRead: z.string().optional(),
    officialCacheWrite: z.string().optional(),
    saleMultiplier: z.string().optional(),
    costMultiplier: z.string().optional(),
  })

export type ModelPricingFormValues = z.infer<
  ReturnType<typeof createModelPricingSchema>
>

export type PricingMode = 'per-token' | 'per-request' | 'tiered_expr'

export type LaneKey =
  | 'completion'
  | 'cache'
  | 'createCache'
  | 'image'
  | 'audioInput'
  | 'audioOutput'

export type ModelRatioData = {
  name: string
  price?: string
  ratio?: string
  cacheRatio?: string
  createCacheRatio?: string
  completionRatio?: string
  imageRatio?: string
  audioRatio?: string
  audioCompletionRatio?: string
  billingMode?: PricingMode
  billingExpr?: string
  requestRuleExpr?: string
  // 成本价（平台买入价 $/1M tokens），与售价完全分离，仅超管可写。
  // 编辑态用 string（输入框），序列化时 parseFloat 成 number。
  costInput?: string
  costOutput?: string
  costCache?: string
  // 倍率定价模式（multiplier）专用：官方价 $/1M + 销售/成本倍率（编辑态 string）。
  // 仅 multiplier 模式填充；提交时 convertMultiplierToRatioData 换算成上面的 ratio/cost。
  officialInput?: string
  officialOutput?: string
  officialCacheRead?: string
  officialCacheWrite?: string
  saleMultiplier?: string
  costMultiplier?: string
  // v2 融入式: per-request/tiered_expr 的官方价 + 派生成本/表达式 + 模式标记(还原用)
  officialRequestPrice?: string
  officialExpr?: string
  costPerRequest?: string
  costExpr?: string
  pricingSourceMode?: PricingMode
}

// 后端 option key "ModelCost" 的 JSON 值结构（嵌套对象 map），对应
// setting/ratio_setting/model_cost.go 的 ModelCostInfo（float64 $/1M tokens）。
// 售价是扁平 Record<string, number>，成本是 Record<string, ModelCostInfo>，结构不同，不能混用。
export type ModelCostInfo = {
  input_cost_per_m: number
  output_cost_per_m: number
  cache_cost_per_m?: number
  cost_per_request?: number
  cost_expr?: string
}

// 后端 option key "ModelPricingSource" 的 JSON 值结构（嵌套对象 map），对应
// setting/ratio_setting/model_pricing_source.go 的 ModelPricingSource。
// 倍率定价模式的编辑态来源：官方价(4项 $/1M) + 销售倍率 + 成本倍率。
// 仅用于前端 UI 还原，计费引擎不读（计费仍走 ModelRatio/ModelCost）。
export type ModelPricingSource = {
  official_input: number
  official_output: number
  official_cache_read: number
  official_cache_write: number
  official_request_price?: number
  official_expr?: string
  sale_multiplier: number
  cost_multiplier: number
}

export type PreviewRow = {
  key: string
  label: string
  value: string
  multiline?: boolean
}

export const numericDraftRegex = /^(\d+(\.\d*)?|\.\d*)?$/

export const EMPTY_LANE_PRICES: Record<LaneKey, string> = {
  completion: '',
  cache: '',
  createCache: '',
  image: '',
  audioInput: '',
  audioOutput: '',
}

export const EMPTY_LANE_ENABLED: Record<LaneKey, boolean> = {
  completion: false,
  cache: false,
  createCache: false,
  image: false,
  audioInput: false,
  audioOutput: false,
}

export const ratioFieldByLane: Record<LaneKey, keyof ModelPricingFormValues> = {
  completion: 'completionRatio',
  cache: 'cacheRatio',
  createCache: 'createCacheRatio',
  image: 'imageRatio',
  audioInput: 'audioRatio',
  audioOutput: 'audioCompletionRatio',
}

export const laneConfigs: Array<{
  key: LaneKey
  titleKey: string
  descriptionKey: string
  placeholder: string
}> = [
  {
    key: 'completion',
    titleKey: 'Completion price',
    descriptionKey: 'Output token price for generated tokens.',
    placeholder: '15',
  },
  {
    key: 'cache',
    titleKey: 'Cache read price',
    descriptionKey: 'Token price for cache reads.',
    placeholder: '0.3',
  },
  {
    key: 'createCache',
    titleKey: 'Cache write price',
    descriptionKey: 'Token price for creating cache entries.',
    placeholder: '3.75',
  },
  {
    key: 'image',
    titleKey: 'Image input price',
    descriptionKey: 'Token price for image input.',
    placeholder: '2.5',
  },
  {
    key: 'audioInput',
    titleKey: 'Audio input price',
    descriptionKey: 'Token price for audio input.',
    placeholder: '3.81',
  },
  {
    key: 'audioOutput',
    titleKey: 'Audio output price',
    descriptionKey: 'Token price for audio output.',
    placeholder: '15.11',
  },
]

export function hasValue(value: unknown): boolean {
  return (
    value !== '' && value !== null && value !== undefined && value !== false
  )
}

export function toNumberOrNull(value: unknown): number | null {
  if (!hasValue(value) && value !== 0) return null
  const num = Number(value)
  return Number.isFinite(num) ? num : null
}

function ratioToBasePrice(ratio: unknown): string {
  const num = toNumberOrNull(ratio)
  if (num === null) return ''
  return formatPricingNumber(num * 2)
}

function deriveLanePrice(
  ratio: unknown,
  denominator: unknown,
  fallback = ''
): string {
  const ratioNumber = toNumberOrNull(ratio)
  const denominatorNumber = toNumberOrNull(denominator)
  if (ratioNumber === null || denominatorNumber === null) return fallback
  return formatPricingNumber(ratioNumber * denominatorNumber)
}

export function createInitialLaneState(data?: ModelRatioData | null) {
  if (!data) {
    return {
      promptPrice: '',
      prices: { ...EMPTY_LANE_PRICES },
      enabled: { ...EMPTY_LANE_ENABLED },
    }
  }

  const promptPrice = ratioToBasePrice(data.ratio)
  const audioInputPrice = deriveLanePrice(data.audioRatio, promptPrice)
  const prices: Record<LaneKey, string> = {
    completion: deriveLanePrice(data.completionRatio, promptPrice),
    cache: deriveLanePrice(data.cacheRatio, promptPrice),
    createCache: deriveLanePrice(data.createCacheRatio, promptPrice),
    image: deriveLanePrice(data.imageRatio, promptPrice),
    audioInput: audioInputPrice,
    audioOutput: deriveLanePrice(data.audioCompletionRatio, audioInputPrice),
  }

  return {
    promptPrice,
    prices,
    enabled: {
      completion: hasValue(data.completionRatio),
      cache: hasValue(data.cacheRatio),
      createCache: hasValue(data.createCacheRatio),
      image: hasValue(data.imageRatio),
      audioInput: hasValue(data.audioRatio),
      audioOutput: hasValue(data.audioCompletionRatio),
    },
  }
}

// scaleBillingExprCoeffs 把分段表达式里「变量*系数」的系数 × mul, 用于 tiered_expr 倍率化。
// 配合 splitBillingExprAndRequestRules/combineBillingExpr 保护 request rule 乘子。
// 不命中 len <= N 等条件(无 变量*系数 模式), 安全。
export function scaleBillingExprCoeffs(expr: string, mul: number): string {
  if (!expr || !Number.isFinite(mul) || mul === 1) return expr
  const { billingExpr, requestRuleExpr } = splitBillingExprAndRequestRules(expr)
  if (!billingExpr) return expr
  const scaled = billingExpr.replace(
    BILLING_VAR_REGEX,
    (_m, variable: string, coeff: string) =>
      `${variable} * ${formatPricingNumber(Number(coeff) * mul)}`
  )
  return combineBillingExpr(scaled, requestRuleExpr)
}

// multiplier 模式编辑态（融入式：仅全局倍率，官方价在各 Tab state 里）。
export type MultiplierLaneState = {
  saleMultiplier: string
  costMultiplier: string
  isApproximate: boolean
}

// 从 ModelRatioData 还原全局倍率(融入式: 只返回 saleMul/costMul)。
// 各 Tab 的官方价由 sheet useEffect 从 ModelPricingSource.official_* 精确还原
// (避免 tiered_expr 系数指数膨胀——必须读 official_expr 而非售价 billingExpr)。
// 老数据无 source 时 saleMul≈1, 标 isApproximate。
export function createInitialMultiplierState(
  data?: ModelRatioData | null
): MultiplierLaneState {
  if (!data) {
    return { saleMultiplier: '', costMultiplier: '', isApproximate: false }
  }
  if (hasValue(data.saleMultiplier) || hasValue(data.costMultiplier)) {
    return {
      saleMultiplier: data.saleMultiplier || '',
      costMultiplier: data.costMultiplier || '',
      isApproximate: false,
    }
  }
  // 老数据无 source: saleMul≈1, 标 Approximate(用户保存后 source 建立)
  return { saleMultiplier: '1', costMultiplier: '', isApproximate: true }
}

export function buildPreviewRows(
  values: ModelPricingFormValues,
  mode: PricingMode,
  billingExpr: string,
  requestRuleExpr: string,
  promptPrice: string,
  lanePrices: Record<LaneKey, string>,
  laneEnabled: Record<LaneKey, boolean>,
  saleMultiplier: string,
  costMultiplier: string,
  t: (key: string) => string
): PreviewRow[] {
  // 成本与计费模式无关，所有模式都在预览末尾展示。
  const costRows: PreviewRow[] = [
    {
      key: 'costInput',
      label: t('Input cost'),
      value: values.costInput ? `$${values.costInput}` : t('Empty'),
    },
    {
      key: 'costOutput',
      label: t('Output cost'),
      value: values.costOutput ? `$${values.costOutput}` : t('Empty'),
    },
    {
      key: 'costCache',
      label: t('Cache cost'),
      value: values.costCache ? `$${values.costCache}` : t('Empty'),
    },
  ]

  if (mode === 'tiered_expr') {
    const effectiveExpr = combineBillingExpr(billingExpr, requestRuleExpr)
    const saleMul = toNumberOrNull(saleMultiplier)
    const costMul = toNumberOrNull(costMultiplier)
    const saleExpr =
      saleMul !== null
        ? combineBillingExpr(
            scaleBillingExprCoeffs(billingExpr, saleMul),
            requestRuleExpr
          )
        : ''
    const costExpr =
      costMul !== null ? scaleBillingExprCoeffs(billingExpr, costMul) : ''
    return [
      { key: 'mode', label: 'BillingMode', value: 'tiered_expr' },
      {
        key: 'expr',
        label: t('Official expression'),
        value: effectiveExpr || t('Empty'),
        multiline: true,
      },
      ...(saleExpr
        ? [
            {
              key: 'saleExpr',
              label: t('Sale expression'),
              value: saleExpr,
              multiline: true,
            },
          ]
        : []),
      ...(costExpr
        ? [
            {
              key: 'costExpr',
              label: t('Cost expression'),
              value: costExpr,
              multiline: true,
            },
          ]
        : []),
      ...costRows,
    ]
  }

  if (mode === 'per-request') {
    return [
      {
        key: 'price',
        label: 'ModelPrice',
        value: values.price || t('Empty'),
      },
      ...costRows,
    ]
  }

  return [
    {
      key: 'inputPrice',
      label: t('Input price'),
      value: promptPrice ? `$${promptPrice}` : t('Empty'),
    },
    {
      key: 'completion',
      label: t('Completion price'),
      value:
        laneEnabled.completion && lanePrices.completion
          ? `$${lanePrices.completion}`
          : t('Empty'),
    },
    {
      key: 'cache',
      label: t('Cache read price'),
      value:
        laneEnabled.cache && lanePrices.cache
          ? `$${lanePrices.cache}`
          : t('Empty'),
    },
    {
      key: 'createCache',
      label: t('Cache write price'),
      value:
        laneEnabled.createCache && lanePrices.createCache
          ? `$${lanePrices.createCache}`
          : t('Empty'),
    },
    {
      key: 'image',
      label: t('Image input price'),
      value:
        laneEnabled.image && lanePrices.image
          ? `$${lanePrices.image}`
          : t('Empty'),
    },
    {
      key: 'audio',
      label: t('Audio input price'),
      value:
        laneEnabled.audioInput && lanePrices.audioInput
          ? `$${lanePrices.audioInput}`
          : t('Empty'),
    },
    {
      key: 'audioCompletion',
      label: t('Audio output price'),
      value:
        laneEnabled.audioOutput && lanePrices.audioOutput
          ? `$${lanePrices.audioOutput}`
          : t('Empty'),
    },
    ...costRows,
  ]
}

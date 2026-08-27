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
import { useState, useEffect } from 'react'
import { Crown, CalendarClock, Package } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { DEFAULT_CURRENCY_CONFIG } from '@/stores/system-config-store'
import { formatQuota } from '@/lib/format'
import { useSystemConfig } from '@/hooks/use-system-config'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Dialog } from '@/components/dialog'
import { GroupBadge } from '@/components/group-badge'
import {
  paySubscriptionStripe,
  paySubscriptionCreem,
  paySubscriptionEpay,
  paySubscriptionWaffoPancake,
  paySubscriptionBalance,
} from '../../api'
import { formatDuration, formatResetPeriod } from '../../lib'
import type { PlanRecord, SubscriptionRenewalPreview } from '../../types'

interface PaymentMethod {
  type: string
  name?: string
  fee_rate?: number
}

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  plan: PlanRecord | null
  enableStripe?: boolean
  enableCreem?: boolean
  enableWaffoPancake?: boolean
  enableOnlineTopUp?: boolean
  epayMethods?: PaymentMethod[]
  purchaseLimit?: number
  purchaseCount?: number
  userQuota?: number
  renewalPreview?: SubscriptionRenewalPreview | null
  onPurchaseSuccess?: () => void | Promise<void>
}

export function SubscriptionPurchaseDialog(props: Props) {
  const { t } = useTranslation()
  const { currency } = useSystemConfig()
  const [paying, setPaying] = useState(false)
  const [selectedEpayMethod, setSelectedEpayMethod] = useState('')

  useEffect(() => {
    let active = true
    void Promise.resolve().then(() => {
      if (!active) return
      if (props.open && props.epayMethods && props.epayMethods.length > 0) {
        setSelectedEpayMethod(props.epayMethods[0].type)
      } else if (!props.open) {
        setSelectedEpayMethod('')
      }
    })
    return () => {
      active = false
    }
  }, [props.open, props.epayMethods])

  const plan = props.plan?.plan
  if (!plan) return null

  const hasStripe = props.enableStripe && !!plan.stripe_price_id
  const hasCreem = props.enableCreem && !!plan.creem_product_id
  const hasWaffoPancake =
    props.enableWaffoPancake && !!plan.waffo_pancake_product_id
  const hasEpay =
    props.enableOnlineTopUp && (props.epayMethods || []).length > 0
  const hasAnyPayment = hasStripe || hasCreem || hasWaffoPancake || hasEpay
  const selectedEpayMethodLabel =
    (props.epayMethods || []).find((m) => m.type === selectedEpayMethod)
      ?.name ||
    selectedEpayMethod ||
    t('Select payment method')
  const totalAmount = Number(plan.total_amount || 0)
  const price = Number(plan.price_amount || 0).toFixed(2)
  const selectedEpayFeeRate = Number(
    (props.epayMethods || []).find((m) => m.type === selectedEpayMethod)
      ?.fee_rate || 0
  )
  const selectedEpayFee =
    selectedEpayFeeRate > 0
      ? Math.round(Number(plan.price_amount || 0) * selectedEpayFeeRate) / 100
      : 0
  const productAmount = Number(plan.price_amount || 0)
  const selectedEpayTotal =
    Math.round((productAmount + selectedEpayFee) * 100) / 100
  const quotaPerUnit =
    currency?.quotaPerUnit && currency.quotaPerUnit > 0
      ? currency.quotaPerUnit
      : DEFAULT_CURRENCY_CONFIG.quotaPerUnit
  const balanceCost = Math.max(
    0,
    Math.ceil(Number(plan.price_amount || 0) * quotaPerUnit)
  )
  const userQuota = Math.max(0, Number(props.userQuota || 0))
  const allowBalancePay = plan.allow_balance_pay !== false
  const insufficientBalance = userQuota < balanceCost
  const limitReached =
    !props.renewalPreview &&
    (props.purchaseLimit || 0) > 0 &&
    (props.purchaseCount || 0) >= (props.purchaseLimit || 0)
  const payRequest = {
    plan_id: plan.id,
    ...(props.renewalPreview
      ? {
          renew_from_subscription_id: props.renewalPreview.from_subscription.id,
        }
      : {}),
  }

  const handlePayStripe = async () => {
    setPaying(true)
    try {
      const res = await paySubscriptionStripe(payRequest)
      if (res.message === 'success' && res.data?.pay_link) {
        window.open(res.data.pay_link, '_blank')
        toast.success(t('Payment page opened'))
        props.onOpenChange(false)
      } else {
        toast.error(
          res.message && res.message !== 'success'
            ? res.message
            : t('Payment request failed')
        )
      }
    } catch {
      toast.error(t('Payment request failed'))
    } finally {
      setPaying(false)
    }
  }

  const handlePayCreem = async () => {
    setPaying(true)
    try {
      const res = await paySubscriptionCreem(payRequest)
      if (res.message === 'success' && res.data?.checkout_url) {
        window.open(res.data.checkout_url, '_blank')
        toast.success(t('Payment page opened'))
        props.onOpenChange(false)
      } else {
        toast.error(
          res.message && res.message !== 'success'
            ? res.message
            : t('Payment request failed')
        )
      }
    } catch {
      toast.error(t('Payment request failed'))
    } finally {
      setPaying(false)
    }
  }

  // In-tab redirect (not window.open) — user-gesture context is lost
  // across the await, so a popup would be blocked. Same as the wallet hook.
  const handlePayWaffoPancake = async () => {
    setPaying(true)
    try {
      const res = await paySubscriptionWaffoPancake(payRequest)
      if (res.message === 'success' && res.data?.checkout_url) {
        toast.success(t('Redirecting to payment page...'))
        window.location.href = res.data.checkout_url
      } else {
        toast.error(
          res.message && res.message !== 'success'
            ? res.message
            : t('Payment request failed')
        )
      }
    } catch {
      toast.error(t('Payment request failed'))
    } finally {
      setPaying(false)
    }
  }

  const isSafari =
    typeof navigator !== 'undefined' &&
    /^((?!chrome|android).)*safari/i.test(navigator.userAgent)

  const handlePayEpay = async () => {
    if (!selectedEpayMethod) {
      toast.error(t('Please select a payment method'))
      return
    }
    setPaying(true)
    try {
      const res = await paySubscriptionEpay({
        ...payRequest,
        payment_method: selectedEpayMethod,
      })
      if (res.message === 'success' && res.url) {
        const form = document.createElement('form')
        form.action = res.url
        form.method = 'POST'
        if (!isSafari) {
          form.target = '_blank'
        }
        Object.entries(res.data || {}).forEach(([key, value]) => {
          const input = document.createElement('input')
          input.type = 'hidden'
          input.name = key
          input.value = String(value)
          form.appendChild(input)
        })
        document.body.appendChild(form)
        form.submit()
        document.body.removeChild(form)
        toast.success(t('Payment initiated'))
        props.onOpenChange(false)
      } else {
        toast.error(
          res.message && res.message !== 'success'
            ? res.message
            : t('Payment request failed')
        )
      }
    } catch {
      toast.error(t('Payment request failed'))
    } finally {
      setPaying(false)
    }
  }

  const handlePayBalance = async () => {
    if (!allowBalancePay) {
      toast.error(t('This plan does not allow balance redemption'))
      return
    }
    setPaying(true)
    try {
      const res = await paySubscriptionBalance(payRequest)
      if (res.success) {
        toast.success(
          props.renewalPreview
            ? t('Renewal created successfully')
            : t('Subscription purchased successfully')
        )
        void props.onPurchaseSuccess?.()
        props.onOpenChange(false)
      } else {
        toast.error(
          res.message && res.message !== 'success'
            ? res.message
            : t('Payment request failed')
        )
      }
    } catch {
      toast.error(t('Payment request failed'))
    } finally {
      setPaying(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={
        <>
          <Crown className='h-5 w-5' />
          {props.renewalPreview
            ? t('Confirm Subscription Renewal')
            : t('Purchase Subscription')}
        </>
      }
      contentClassName='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'
      titleClassName='flex items-center gap-2'
      contentHeight='auto'
      bodyClassName='space-y-4'
    >
      <div className='space-y-3 sm:space-y-4'>
        {props.renewalPreview && (
          <Alert>
            <AlertDescription className='space-y-1.5'>
              <p className='font-medium'>
                {t(
                  'Renewal creates a new subscription instance using the current plan terms shown below.'
                )}
              </p>
              <p>
                {t('New instance starts at')}{' '}
                {new Date(
                  props.renewalPreview.start_time * 1000
                ).toLocaleString()}
                ，
                {t('and inherits the API Key bindings visible in the preview.')}
              </p>
              {props.renewalPreview.is_replacement && (
                <p className='text-warning'>
                  {t(
                    'The original plan is no longer renewable. The system will use its configured replacement plan.'
                  )}
                </p>
              )}
            </AlertDescription>
          </Alert>
        )}
        <div className='bg-muted/50 space-y-2.5 rounded-lg border p-3 sm:space-y-3 sm:p-4'>
          <div className='flex justify-between'>
            <span className='text-muted-foreground text-sm'>
              {t('Plan Name')}
            </span>
            <span className='max-w-[200px] truncate text-sm font-medium'>
              {plan.title}
            </span>
          </div>
          <div className='flex items-center justify-between'>
            <span className='text-muted-foreground text-sm'>
              {t('Validity Period')}
            </span>
            <span className='flex items-center gap-1 text-sm'>
              <CalendarClock className='h-3.5 w-3.5' />
              {formatDuration(plan, t)}
            </span>
          </div>
          {formatResetPeriod(plan, t) !== t('No Reset') && (
            <div className='flex justify-between'>
              <span className='text-muted-foreground text-sm'>
                {t('Reset Period')}
              </span>
              <span className='text-sm'>{formatResetPeriod(plan, t)}</span>
            </div>
          )}
          <div className='flex items-center justify-between'>
            <span className='text-muted-foreground text-sm'>
              {t('Received amount')}
            </span>
            <span className='flex items-center gap-1 text-sm'>
              <Package className='h-3.5 w-3.5' />
              {totalAmount > 0 ? formatQuota(totalAmount) : t('Unlimited')}
            </span>
          </div>
          {plan.upgrade_group && (
            <div className='flex items-center justify-between'>
              <span className='text-muted-foreground text-sm'>
                {t('Upgrade Group')}
              </span>
              <GroupBadge group={plan.upgrade_group} />
            </div>
          )}
          {plan.allowed_group && (
            <div className='flex items-center justify-between'>
              <span className='text-muted-foreground text-sm'>
                {t('Available Endpoint')}
              </span>
              <GroupBadge group={plan.allowed_group} />
            </div>
          )}
          {props.renewalPreview && (
            <div className='flex justify-between gap-3'>
              <span className='text-muted-foreground text-sm'>
                {t('New validity period')}
              </span>
              <span className='text-right text-sm'>
                {new Date(
                  props.renewalPreview.start_time * 1000
                ).toLocaleDateString()}{' '}
                –{' '}
                {new Date(
                  props.renewalPreview.end_time * 1000
                ).toLocaleDateString()}
              </span>
            </div>
          )}
          {props.renewalPreview &&
            props.renewalPreview.binding_changes.length > 0 && (
              <div className='space-y-1.5 rounded-md border p-2.5'>
                <div className='text-xs font-medium'>
                  {t('API Keys carried to the new instance')} (
                  {props.renewalPreview.binding_changes.length})
                </div>
                {props.renewalPreview.binding_changes.map((change) => (
                  <div
                    key={change.token_id}
                    className='text-muted-foreground flex items-center justify-between gap-3 text-xs'
                  >
                    <span className='truncate'>{change.token_name}</span>
                    <span className='shrink-0'>
                      {change.from_group}
                      {change.from_group !== change.to_group
                        ? ` → ${change.to_group}`
                        : ''}
                    </span>
                  </div>
                ))}
                {props.renewalPreview.binding_changes.some(
                  (change) => change.from_group !== change.to_group
                ) && (
                  <p className='text-warning text-xs'>
                    {t(
                      'The renewed plan uses a different group. These API Keys will switch group and instance together when the renewal becomes active.'
                    )}
                  </p>
                )}
              </div>
          )}
          <Separator />
          {hasEpay && selectedEpayMethod && (
            <>
              <div className='flex items-center justify-between text-sm'>
                <span className='text-muted-foreground'>
                  {t('Product amount')}
                </span>
                <span>${productAmount.toFixed(2)}</span>
              </div>
              <div className='flex items-center justify-between text-sm'>
                <span className='text-muted-foreground'>
                  {t('Payment fee')}
                  {selectedEpayFeeRate > 0
                    ? ` (${selectedEpayFeeRate}%)`
                    : ''}
                </span>
                <span>${selectedEpayFee.toFixed(2)}</span>
              </div>
              <div className='flex items-center justify-between'>
                <span className='text-sm font-medium'>
                  {t('Actual payment')}
                </span>
                <span className='text-primary text-lg font-bold'>
                  ${selectedEpayTotal.toFixed(2)}
                </span>
              </div>
            </>
          )}
          {(!hasEpay || !selectedEpayMethod) && (
            <div className='flex items-center justify-between text-sm'>
              <span className='text-sm font-medium'>{t('Amount Due')}</span>
              <span className='text-primary text-lg font-bold'>${price}</span>
            </div>
          )}
        </div>

        {limitReached && (
          <Alert variant='destructive'>
            <AlertDescription>
              {t('Purchase limit reached')} ({props.purchaseCount}/
              {props.purchaseLimit})
            </AlertDescription>
          </Alert>
        )}

        <div className='flex flex-col gap-2 rounded-md border p-3'>
          <div className='flex items-center justify-between gap-2 text-xs'>
            <span className='text-muted-foreground'>{t('Required')}</span>
            <span>{formatQuota(balanceCost)}</span>
          </div>
          <div className='flex items-center justify-between gap-2 text-xs'>
            <span className='text-muted-foreground'>{t('Available')}</span>
            <span>{formatQuota(userQuota)}</span>
          </div>
          {!allowBalancePay ? (
            <Alert variant='destructive'>
              <AlertDescription>
                {t('This plan does not allow balance redemption')}
              </AlertDescription>
            </Alert>
          ) : (
            insufficientBalance && (
              <Alert variant='destructive'>
                <AlertDescription>{t('Insufficient balance')}</AlertDescription>
              </Alert>
            )
          )}
          <Button
            variant='outline'
            onClick={handlePayBalance}
            disabled={
              paying || limitReached || !allowBalancePay || insufficientBalance
            }
          >
            {t('Pay with Balance')}
          </Button>
        </div>

        {hasAnyPayment && (
          <div className='space-y-3'>
            <p className='text-muted-foreground text-xs'>
              {t('Select payment method')}
            </p>
            {(hasStripe || hasCreem || hasWaffoPancake) && (
              <div className='grid grid-cols-2 gap-2 sm:flex'>
                {hasStripe && (
                  <Button
                    variant='outline'
                    className='flex-1'
                    onClick={handlePayStripe}
                    disabled={paying || limitReached}
                  >
                    Stripe
                  </Button>
                )}
                {hasCreem && (
                  <Button
                    variant='outline'
                    className='flex-1'
                    onClick={handlePayCreem}
                    disabled={paying || limitReached}
                  >
                    Creem
                  </Button>
                )}
                {hasWaffoPancake && (
                  <Button
                    variant='outline'
                    className='flex-1'
                    onClick={handlePayWaffoPancake}
                    disabled={paying || limitReached}
                  >
                    Waffo Pancake
                  </Button>
                )}
              </div>
            )}
            {hasEpay && (
              <div className='grid grid-cols-[minmax(0,1fr)_auto] gap-2'>
                <Select
                  items={[
                    ...(props.epayMethods || []).map((m) => ({
                      value: m.type,
                      label: m.name || m.type,
                    })),
                  ]}
                  value={selectedEpayMethod}
                  onValueChange={(v) => v !== null && setSelectedEpayMethod(v)}
                  disabled={limitReached}
                >
                  <SelectTrigger className='flex-1'>
                    <SelectValue>{selectedEpayMethodLabel}</SelectValue>
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {(props.epayMethods || []).map((m) => (
                        <SelectItem key={m.type} value={m.type}>
                          {m.name || m.type}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <Button
                  onClick={handlePayEpay}
                  disabled={paying || !selectedEpayMethod || limitReached}
                >
                  {t('Pay')}
                </Button>
              </div>
            )}
          </div>
        )}
      </div>
    </Dialog>
  )
}
